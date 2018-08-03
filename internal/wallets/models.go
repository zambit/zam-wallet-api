package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal"
)

// WalletWithBalance represents wallet with balance
type WalletWithBalance struct {
	queries.Wallet

	// Balance of the wallet represented using high-precision decimal type
	Balance *decimal.Big
}
