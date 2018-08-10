package balance

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
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
	addressBalance, err := b.Coordinator.ObserverWithCtx(ctx, wallet.Coin.ShortName).Balance(wallet.Address)
	if err != nil {
		return
	}

	// calculate sum of wallet txs
	txsSum, err := b.ProcessingApi.GetTxsesSum(wallet)
	if err != nil {
		return
	}

	// get total balance
	if txsSum != nil {
		balance = new(decimal.Big).Add(addressBalance, txsSum)
	} else {
		balance = addressBalance
	}

	span.LogKV("txs_sum", txsSum, "address_balance", addressBalance, "balance", balance)

	return
}
