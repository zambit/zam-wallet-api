package balance

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/ericlagergren/decimal"
	ot "github.com/opentracing/opentracing-go"
)

// IBalance implementation
type Balance struct {
	Coordinator   nodes.ICoordinator
	ProcessingApi processing.IApi
}

// New
func New(coordinator nodes.ICoordinator, api processing.IApi) *Balance {
	return &Balance{
		Coordinator:   coordinator,
		ProcessingApi: api,
	}
}

// AccountBalance implements IBalance
func (b *Balance) AccountBalanceCtx(ctx context.Context, coinName string) (balance *decimal.Big, err error) {
	return b.Coordinator.AccountObserverWithCtx(ctx, coinName).GetBalance()
}

// TotalWalletBalance implements IBalance
func (b *Balance) TotalWalletBalanceCtx(ctx context.Context, wallet *queries.Wallet) (balance *decimal.Big, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "total_wallet_balance")
	defer span.Finish()

	span.LogKV("wallet_id", wallet.ID, "coin", wallet.Coin.ShortName)

	// query address balance using node service
	var addressBalance *decimal.Big
	trace.InsideSpan(ctx, "get_node_address_balance", func(ctx context.Context, span ot.Span) {
		addressBalance, err = b.Coordinator.ObserverWithCtx(ctx, wallet.Coin.ShortName).Balance(wallet.Address)
		span.LogKV("address_balance", addressBalance)
	})
	if err != nil {
		return
	}

	// calculate sum of wallet txs
	var txsSum *decimal.Big
	trace.InsideSpan(ctx, "get_wallet_txs_sum", func(ctx context.Context, span ot.Span) {
		txsSum, err = b.ProcessingApi.GetTxsesSum(ctx, wallet)
		span.LogKV("txs_sum", txsSum)
	})
	if err != nil {
		return
	}

	// get total balance
	if txsSum != nil {
		balance = new(decimal.Big).Add(addressBalance, txsSum)
	} else {
		balance = addressBalance
	}

	span.LogKV("balance", balance)

	return
}
