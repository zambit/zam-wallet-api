package helpers

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal"
)

// IBalance helpers which provides access to balances which calculated from different sources
type IBalance interface {
	// AccountBalanceCtx returns total available to spend balance
	AccountBalanceCtx(ctx context.Context, coinName string) (balance *decimal.Big, err error)

	// TotalWalletBalanceCtx returns balance calculated as sum of value associated with wallet address and wallet txs sum
	TotalWalletBalanceCtx(ctx context.Context, wallet *queries.Wallet) (balance *decimal.Big, err error)
}
