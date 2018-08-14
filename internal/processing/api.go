package processing

import (
	"context"
	"errors"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/jinzhu/gorm"
	. "github.com/opentracing/opentracing-go"
)

var (
	// ErrNoOneTxAwaitsWallet if no one transaction affected.
	ErrNoOneTxAwaitsWallet = errors.New("processing: no one tx awaits wallet")

	// ErrTxAmountToBig returned when tx exceed amount threshold
	ErrTxAmountToBig = errors.New("processing: tx is exceed amount threshold")

	// ErrInsufficientFunds anyone knows what this error means.
	ErrInsufficientFunds = errors.New("processing: insufficient funds")

	// ErrZeroAmount when amount is zero
	ErrZeroAmount = errors.New("processing: zero amount")

	// ErrNegativeAmount when negative amount passed
	ErrNegativeAmount = errors.New("processing: negative amount")
)

type InternalTxRecipientType int

const (
	InternalTxWalletRecipient = iota
	InternalTxPhoneRecipient
)

// InternalTxRecipient describes
type InternalTxRecipient struct {
	Type   InternalTxRecipientType
	Phone  string
	Wallet *queries.Wallet
}

// IApi represents wallet transaction operations and implements simplified processing center, which able to
// process internal transactions, track their states and waits until specific user creates wallet.
type IApi interface {
	// SendInternal
	SendInternal(ctx context.Context, wallet *queries.Wallet, recipient InternalTxRecipient, amount *decimal.Big) (newTx *Tx, err error)

	// GetTxsesSum get sum of outgoing and incoming transactions for specified wallet
	GetTxsesSum(ctx context.Context, wallet *queries.Wallet) (sum *decimal.Big, err error)

	// NotifyUserCreatesWallet lookups pending transactions which waits wallet of this user and perform transactions.
	// Returns ErrNoOneTxAwaitsWallet if no one affected.
	// May return ErrNoSuchWallet.
	NotifyUserCreatesWallet(ctx context.Context, wallet *queries.Wallet) error
}

// Api is IApi implementation
type Api struct {
	database      *gorm.DB
	coordinator   nodes.ICoordinator
	balanceHelper helpers.IBalance
}

// New
func New(db *gorm.DB, coordinator nodes.ICoordinator, balanceHelper helpers.IBalance) IApi {
	return &Api{
		database:      db,
		coordinator:   coordinator,
		balanceHelper: balanceHelper,
	}
}

// SendByPhone is IApi implementation
func (api *Api) SendInternal(ctx context.Context, wallet *queries.Wallet, recipient InternalTxRecipient, amount *decimal.Big) (newTx *Tx, err error) {
	span, ctx := StartSpanFromContext(ctx, "send_internal")
	defer span.Finish()

	span.LogKV(
		"from_wallet_id", wallet.ID,
		"to_wallet_id", recipient.Wallet.ID,
		"coin", wallet.Coin.ShortName,
		"amount", amount,
	)

	// check amount, it should be greater the zero
	switch amount.Sign() {
	case 0:
		err = ErrZeroAmount
		return
	case -1:
		err = ErrNegativeAmount
		return
	}

	// create new tx in transaction for future select for update
	err = db.TransactionCtx(ctx, api.database, func(ctx context.Context, tx *gorm.DB) error {
		// query status explicitly, no clear way with gorm :(
		var stateModel TxStatus
		err = tx.Model(&stateModel).Where("name = ?", TxStateJustCreated).First(&stateModel).Error
		if err != nil {
			return err
		}

		// create new wallet right in done state
		pTx := &Tx{
			FromWallet: wallet,
			ToWallet:   recipient.Wallet,
			Type:       TxInternal,
			Amount:     &postgres.Decimal{V: amount},
			Status:     &stateModel,
		}
		err = tx.Create(pTx).Error
		if err != nil {
			return err
		}

		newTx = pTx
		span.LogKV("new_tx_id", newTx.ID)

		// preform steps
		newTx, err = StepTx(ctx, api.database, newTx, &StepResources{
			Coordinator:   api.coordinator,
			BalanceHelper: api.balanceHelper,
		})

		return nil
	})
	if err != nil {
		return
	}

	if err != nil {
		span.LogKV("err", err)
	}
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
			span.LogKV("msg", "no txs for this wallet")
		}
		return nil
	})

	if err != nil {
		span.LogKV("err", err)
	}
	span.LogKV("sum", sum)
	return
}

// NotifyUserCreatesWallet implements IApi interface
func (*Api) NotifyUserCreatesWallet(ctx context.Context, wallet *queries.Wallet) error {
	panic("implement me")
}
