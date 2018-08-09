package processing

import (
	"context"
	"errors"
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
)

// IApi represents wallet transaction operations and implements simplified processing center, which able to
// process internal transactions, track their states and waits until specific user creates wallet.
type IApi interface {
	// WithCtx attach context to the next method calls. Returned value must not outlive given context.
	WithCtx(ctx context.Context) IApi

	// SendByPhone sends internal transaction determining recipient wallet by source wallet and dest phone number. If
	// user not exists, transaction will be marked as "pending" and may be continued by `NotifyUserCreatesWallet` call.
	// May return ErrNoSuchWallet.
	SendInternal(wallet queries.Wallet, toWallet queries.Wallet, amount *decimal.Big) (newTx *Tx, err error)

	// GetTxsesSum get sum of outgoing and incoming transactions for specified wallet
	GetTxsesSum(wallet queries.Wallet) (sum *decimal.Big, err error)

	// NotifyUserCreatesWallet lookups pending transactions which waits wallet of this user and perform transactions.
	// Returns ErrNoOneTxAwaitsWallet if no one affected.
	// May return ErrNoSuchWallet.
	NotifyUserCreatesWallet(wallet queries.Wallet) error
}

// Api is IApi implementation
type Api struct {
	ctx context.Context

	d           *gorm.DB
	coordinator nodes.ICoordinator
}

// New
func New(db *gorm.DB, coordinator nodes.ICoordinator) IApi {
	return &Api{
		d:           db,
		coordinator: coordinator,
		ctx:         context.Background(),
	}
}

// WithCtx
func (api *Api) WithCtx(ctx context.Context) IApi {
	cc := *api
	cc.ctx = ctx
	return &cc
}

// SendByPhone is IApi implementation
func (api *Api) SendInternal(wallet queries.Wallet, toWallet queries.Wallet, amount *decimal.Big) (newTx *Tx, err error) {
	span, ctx := StartSpanFromContext(api.ctx, "send_by_phone")
	defer span.Finish()

	span.LogKV(
		"from_wallet_id", wallet.ID,
		"to_wallet_id", toWallet.ID,
		"coin", wallet.Coin.ShortName,
		"amount", amount,
	)

	err = TransactionCtx(ctx, api.d, func(ctx context.Context, tx *gorm.DB) (err error) {
		span := SpanFromContext(ctx)
		defer span.Finish()

		// query wallet balance, tx amount should not
		generalBalance, err := api.coordinator.AccountObserverWithCtx(ctx, wallet.Coin.ShortName).GetBalance()
		if err != nil {
			return
		}

		// tx amount should not exceed node balance, return amount to big err in other case
		span.LogKV("account balance", generalBalance)
		if generalBalance.Cmp(amount) < 0 {
			return ErrTxAmountToBig
		}

		// query status explicitly, no other way here :(
		var status TxStatus
		err = tx.Where(TxStatus{Name: "success"}).First(&status).Error
		if err != nil {
			return err
		}

		// create new wallet right in done state
		pTx := &Tx{
			FromWallet: &wallet,
			ToWallet:   &toWallet,
			Type:       TxInternal,
			Amount:     &postgres.Decimal{V: amount},
			Status:     &status,
		}
		err = tx.Create(pTx).Error
		if err != nil {
			return
		}

		newTx = pTx
		span.LogKV("new_tx_id", newTx.ID)

		return
	})
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
func (api *Api) GetTxsesSum(wallet queries.Wallet) (sum *decimal.Big, err error) {
	span, ctx := StartSpanFromContext(api.ctx, "txses_sum")
	defer span.Finish()

	span.LogKV("wallet_id", wallet.ID, "coin", wallet.Coin.ShortName)

	err = TransactionCtx(ctx, api.d, func(ctx context.Context, tx *gorm.DB) error {
		type scanResT struct {
			Sum     *postgres.Decimal
			Income  *postgres.Decimal
			Outcome *postgres.Decimal
		}

		var scanRes scanResT
		rows, err := tx.Raw(aggregateTxsesQuery, wallet.ID).Rows()
		defer rows.Close()

		for rows.Next() {
			err = rows.Scan(&scanRes.Sum, &scanRes.Income, &scanRes.Outcome)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}

		if scanRes.Income != nil && scanRes.Outcome != nil && scanRes.Sum != nil {
			span := SpanFromContext(ctx)
			defer span.Finish()
			sum = scanRes.Sum.V
			span.LogKV("income", scanRes.Income.V, "outcome", scanRes.Outcome.V, "sum", scanRes.Sum.V)
		}

		return nil
	})
	if err != nil {
		span.LogKV("err", err)
	}
	return
}

// NotifyUserCreatesWallet implements IApi interface
func (*Api) NotifyUserCreatesWallet(wallet queries.Wallet) error {
	panic("implement me")
}

//func TransactionCtx
func TransactionCtx(ctx context.Context, db *gorm.DB, cb func(ctx context.Context, tx *gorm.DB) error) error {
	node, cCtx := StartSpanFromContext(ctx, "transaction")
	defer node.Finish()

	tx := db.Begin()
	if tx.Error != nil {
		node.LogKV("open_tx_err", tx.Error)
		return tx.Error
	}
	defer func() {
		p := recover()
		if p != nil {
			node.LogKV("panic", p)
			tx.Rollback()
			panic(p)
		}
	}()

	err := cb(cCtx, tx)
	if err != nil {
		node.LogKV("cb_err", err)
		return err
	}

	return tx.Commit().Error
}
