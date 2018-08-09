package processing

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gopkg.in/src-d/go-kallax.v1"
)

//go:generate kallax gen -e models.go

// TxStatus
type TxStatus struct {
	kallax.Model `table:"tx_statuses"`

	ID   int64 `pk:"autoincr"`
	Name string
}

// TxID
type TxID uint64

// TxType
type TxType string

const (
	TxInternal = "internal"
	TxExternal = "external"
)

// Tx represents database transaction row
type Tx struct {
	ID           int64
	FromWalletID int64
	FromWallet   *queries.Wallet

	Type TxType

	ToWalletID int64
	ToWallet   *queries.Wallet
	ToAddress  string

	Amount *postgres.Decimal

	StatusID int64
	Status   *TxStatus `gorm:"foreignkey:StatusID"`
}

func (Tx) TableName() string {
	return "txs"
}
