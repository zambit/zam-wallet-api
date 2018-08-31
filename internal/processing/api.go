package processing

import (
	"context"
	"errors"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/jinzhu/gorm"
	. "github.com/opentracing/opentracing-go"
)

var (
	// ErrTxAmountToBig returned when tx exceed amount threshold
	ErrTxAmountToBig = errors.New("processing: tx is exceed maximum amount threshold")

	// ErrInvalidWalletBalance returned when wallet balance exceed general balance
	ErrInvalidWalletBalance = errors.New("processing: wallet balance exceed general balance")

	// ErrSelfTxForbidden returned when self tx detected
	ErrSelfTxForbidden = errors.New("processing: self-tx forbidden")

	// ErrInsufficientFunds anyone knows what this error means.
	ErrInsufficientFunds = errors.New("processing: insufficient funds")

	// ErrZeroAmount when amount is zero
	ErrZeroAmount = errors.New("processing: zero amount")

	// ErrNegativeAmount when negative amount passed
	ErrNegativeAmount = errors.New("processing: negative amount")

	// ErrInvalidAddress external address are invalid
	ErrInvalidAddress = errors.New("processing: invalid external address")
)

type InternalTxRecipientType int

const (
	InternalTxWalletRecipient = iota
	InternalTxPhoneRecipient
)

// InternalTxRecipient describes recipient
type InternalTxRecipient struct {
	t      InternalTxRecipientType
	phone  string
	wallet *queries.Wallet
}

// NewPhoneRecipient sets recipient by phone number (for non-existing recipient wallets)
func NewPhoneRecipient(phone string) InternalTxRecipient {
	return InternalTxRecipient{t: InternalTxPhoneRecipient, phone: phone}
}

// NewWalletRecipient sets recipient by wallet
func NewWalletRecipient(wallet *queries.Wallet) InternalTxRecipient {
	return InternalTxRecipient{t: InternalTxWalletRecipient, wallet: wallet}
}

// IApi represents wallet transaction operations and implements simplified processing center, which able to
// process internal transactions, track their states and waits until specific user creates wallet.
type IApi interface {
	// SendInternal
	SendInternal(
		ctx context.Context,
		wallet *queries.Wallet,
		recipient InternalTxRecipient,
		amount *decimal.Big,
	) (newTx *Tx, err error)

	// SendExternal
	SendExternal(
		ctx context.Context,
		wallet *queries.Wallet,
		address string,
		amount *decimal.Big,
	) (newTx *Tx, err error)

	// GetTxsesSum get sum of outgoing and incoming transactions for specified wallet
	GetTxsesSum(ctx context.Context, wallet *queries.Wallet) (sum *decimal.Big, err error)

	// NotifyUserCreatesWallet lookups pending transactions which waits wallet of this user and perform transactions.
	// Returns ErrNoOneTxAwaitsWallet if no one affected.
	NotifyUserCreatesWallet(ctx context.Context, wallet *queries.Wallet) error
}

// Api is IApi implementation
type Api struct {
	database      *gorm.DB
	balanceHelper helpers.IBalance
	notificator   isc.ITxsEventNotificator
	coordinator   nodes.ICoordinator
}

// New
func New(
	db *gorm.DB,
	balanceHelper helpers.IBalance,
	notificator isc.ITxsEventNotificator,
	cooddinator nodes.ICoordinator,
) IApi {
	return &Api{
		database:      db,
		balanceHelper: balanceHelper,
		notificator:   notificator,
		coordinator:   cooddinator,
	}
}

// SendByPhone implements IApi interface
func (api *Api) SendInternal(
	ctx context.Context,
	wallet *queries.Wallet,
	recipient InternalTxRecipient,
	amount *decimal.Big,
) (newTx *Tx, err error) {
	span, ctx := StartSpanFromContext(ctx, "send_internal")
	defer span.Finish()

	span.LogKV(
		"from_wallet_id", wallet.ID,
		"to_wallet_id", recipient.wallet,
		"to_phone", recipient.phone,
		"coin", wallet.Coin.ShortName,
		"amount", amount,
	)

	// check most common amount errors
	err = checkAmount(amount)
	if err != nil {
		return
	}

	var validationErrs error
	err = db.TransactionCtx(ctx, api.database, func(ctx context.Context, dbTx *gorm.DB) error {
		// query status explicitly, no clear way with gorm :(
		var stateModel TxStatus
		err = dbTx.Model(&stateModel).Where("name = ?", TxStateValidate).First(&stateModel).Error
		if err != nil {
			return err
		}

		// create new wallet
		pTx := &Tx{
			FromWallet: wallet,
			Type:       TxTypeInternal,
			Amount:     &postgres.Decimal{V: amount},
			Status:     &stateModel,
		}
		// fill tx fields depending on recipient type
		switch recipient.t {
		case InternalTxPhoneRecipient:
			pTx.ToPhone = &recipient.phone
		case InternalTxWalletRecipient:
			pTx.ToWallet = recipient.wallet
		}
		err = dbTx.Create(pTx).Error
		if err != nil {
			return err
		}

		newTx = pTx
		span.LogKV("new_tx_id", newTx.ID)

		// preform steps
		newTx, validationErrs, err = StepTx(ctx, dbTx, newTx, api.createExternalResources())

		return err
	})
	if err != nil {
		trace.LogError(span, err)
	} else if validationErrs != nil {
		// don't report as error
		span.LogKV("validation_errs", validationErrs, "message", "validation errs occurs")
		err = validationErrs
	}
	return
}

// SendExternal implements IApi interface
func (api *Api) SendExternal(
	ctx context.Context,
	wallet *queries.Wallet,
	address string,
	amount *decimal.Big,
) (newTx *Tx, err error) {
	err = trace.InsideSpanE(ctx, "send_external", func(ctx context.Context, span Span) error {
		span.LogKV(
			"from_wallet_id", wallet.ID,
			"to_address", address,
			"coin", wallet.Coin.ShortName,
			"amount", amount,
		)

		// check most common amount errors
		err := checkAmount(amount)
		if err != nil {
			return err
		}

		// check self tx
		if wallet.Address == address {
			return ErrSelfTxForbidden
		}

		var validationErrs error
		err = db.TransactionCtx(ctx, api.database, func(ctx context.Context, dbTx *gorm.DB) error {
			// query status explicitly, no clear way with gorm :(
			var stateModel TxStatus
			err = dbTx.Model(&stateModel).Where("name = ?", TxStateValidate).First(&stateModel).Error
			if err != nil {
				return err
			}

			pTx := &Tx{
				FromWallet: wallet,
				Type:       TxTypeExternal,
				Amount:     &postgres.Decimal{V: amount},
				ToAddress:  &address,
				Status:     &stateModel,
			}

			err = dbTx.Create(pTx).Error
			if err != nil {
				return err
			}

			newTx = pTx
			span.LogKV("new_tx_id", pTx.ID)

			// preform steps
			newTx, validationErrs, err = StepTx(ctx, dbTx, pTx, api.createExternalResources())

			return err
		})
		if err != nil {
			return err
		}
		if validationErrs != nil {
			return validationErrs
		}
		return nil
	})
	return
}

const aggregateTxsesQuery = `with income as (select coalesce(sum(txs.amount), 0) as val
                from txs
                where to_wallet_id = $1
                  and type = 'internal'
                  and status_id not in
                      (select id from tx_statuses where name = ANY('{cancel, decline}' :: varchar(30) []))),
     outcome as (select coalesce(sum(txs.amount), 0) as val
                 from txs
                 where from_wallet_id = $1
                   and type = 'internal'
                   and status_id not in
                       (select id from tx_statuses where name = ANY('{cancel, decline}' :: varchar(30) [])))
select income.val - outcome.val as sum, income.val as income, outcome.val as outcome
from income, outcome;`

// GetTxsesSum implements IApi interface
func (api *Api) GetTxsesSum(ctx context.Context, wallet *queries.Wallet) (sum *decimal.Big, err error) {
	span, ctx := StartSpanFromContext(ctx, "txses_sum")
	defer span.Finish()

	span.LogKV("wallet_id", wallet.ID, "coin", wallet.Coin.ShortName)

	err = db.TransactionCtx(ctx, api.database, func(ctx context.Context, tx *gorm.DB) error {
		span := SpanFromContext(ctx)

		var (
			totalSum *postgres.Decimal
			income   *postgres.Decimal
			outcome  *postgres.Decimal
		)
		rows, err := tx.Raw(aggregateTxsesQuery, wallet.ID).Rows()
		// why not uses row? with gorm it sometimes doesn't work as expected, and i don't know why.
		defer rows.Close()
		for rows.Next() {
			err = rows.Scan(&totalSum, &income, &outcome)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}

		// this values mandatory should be logged
		if totalSum != nil {
			sum = totalSum.V
			span.LogKV("income", income.V)
		} else {
			trace.LogMsg(span, "no txs for this wallet")
		}
		return nil
	})

	if err != nil {
		trace.LogError(span, err)
	}
	span.LogKV("sum", sum)
	return
}

// NotifyUserCreatesWallet implements IApi interface
func (api *Api) NotifyUserCreatesWallet(ctx context.Context, wallet *queries.Wallet) (err error) {
	span, ctx := StartSpanFromContext(ctx, "notify_wallet_created")
	defer span.Finish()

	span.LogKV("wallet_id", wallet.ID, "wallet_coin", wallet.Coin.ShortName)

	err = db.TransactionCtx(ctx, api.database, func(ctx context.Context, dbTx *gorm.DB) (err error) {
		// query status explicitly, no clear way with gorm :(
		var stateModel TxStatus
		err = dbTx.Model(&stateModel).Where("name = ?", TxStateAwaitRecipient).First(&stateModel).Error
		if err != nil {
			return err
		}

		// update first
		err = dbTx.Model(&Tx{}).Where(
			`txs.to_phone = ? and
			txs.from_wallet_id in (select id from wallets where coin_id = ?) and
    		txs.status_id = ?`,
			wallet.UserPhone, wallet.CoinID, stateModel.ID,
		).Update("ToWalletID", wallet.ID).Error
		if err != nil {
			return
		}

		// lookup tx which awaits phone number associated with this wallet
		// then select (cannot do it in single query)
		var txsToUpdate []*Tx
		err = dbTx.Model(&Tx{}).Joins(
			"inner join wallets on txs.from_wallet_id = wallets.id",
		).Where(
			`txs.to_phone = ? and
    		wallets.coin_id = ? and
    		txs.status_id = ?`,
			wallet.UserPhone, wallet.CoinID, stateModel.ID,
		).Preload(
			"FromWallet",
		).Preload(
			"FromWallet.Coin",
		).Preload(
			"ToWallet",
		).Preload(
			"Status",
		).Find(&txsToUpdate).Error

		// it's not good idea to transform all dependent transaction in single db transaction, need to elaborate...
		for _, tx := range txsToUpdate {
			// ignore validation errs, TODO should notify user
			_, _, err = StepTx(ctx, dbTx, tx, api.createExternalResources())
		}
		return
	})
	return
}

func (api *Api) createExternalResources() *smResources {
	return &smResources{
		BalanceHelper:      api.balanceHelper,
		TxEventNotificator: api.notificator,
		Coordinator:        api.coordinator,
	}
}

// utils
// checkAmount validates that amount is greater then zero, otherwise returns appropriate error
func checkAmount(amount *decimal.Big) error {
	switch amount.Sign() {
	case 0:
		return ErrZeroAmount
	case -1:
		return ErrNegativeAmount
	default:
		return nil
	}
}
