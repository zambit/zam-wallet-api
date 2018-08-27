package processing

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gopkg.in/src-d/go-kallax.v1"
	"time"
)

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

// Tx types
const (
	TxInternal = "internal"
	TxExternal = "external"
)

// Tx states
const (
	TxStateValidate       = "waiting"
	TxStateDeclined       = "decline"
	TxStateCanceled       = "cancel"
	TxStateAwaitRecipient = "pending"
	TxStateProcessed      = "success"
)

// Tx represents database transaction row
type Tx struct {
	ID           int64
	FromWalletID int64
	FromWallet   *queries.Wallet `gorm:"foreignkey:FromWalletID;association_autoupdate:false;association_autocreate:false"`

	Type TxType

	ToWalletID *int64
	ToWallet   *queries.Wallet `gorm:"foreignkey:ToWalletID;association_autoupdate:false;association_autocreate:false"`
	ToAddress  *string
	ToPhone    *string

	Amount *postgres.Decimal

	CreatedAt time.Time

	StatusID int64
	Status   *TxStatus `gorm:"foreignkey:StatusID;association_autoupdate:false;association_autocreate:false"`
}

func (Tx) TableName() string {
	return "txs"
}

// IsHoldsAmount checks is this txs transaction holds his amount, e.g. such amount of money cannot be spent again
func (tx *Tx) IsHoldsAmount() bool {
	switch tx.Status.Name {
	case TxStateDeclined, TxStateCanceled:
		return false
	default:
		return true
	}
}