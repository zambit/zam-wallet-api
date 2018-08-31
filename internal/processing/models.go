package processing

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"github.com/ericlagergren/decimal/sql/postgres"
	"time"
)

// TxStatus
type TxStatus struct {
	ID   int64
	Name string
}

// TxID
type TxID uint64

// TxType
type TxType string

// Tx types
const (
	TxTypeInternal = "internal"
	TxTypeExternal = "external"
)

// Tx states
const (
	TxStateValidate           = "validation"
	TxStateDeclined           = "decline"
	TxStateCanceled           = "cancel"
	TxStateAwaitRecipient     = "pending"
	TxStateAwaitConfirmations = "waiting"
	TxStateProcessed          = "success"
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

// CoinName returns actual coin short name or <unknown> if model not filled properly
func (tx *Tx) CoinName() string {
	if tx.FromWallet != nil {
		return tx.FromWallet.Coin.ShortName
	}
	return "<unknown>"
}

// StateName returns actual state name or validate for freshly created tx
func (tx *Tx) StateName() string {
	switch {
	case tx.Status != nil:
		return tx.Status.Name
	case tx.Status == nil:
		return TxStateValidate
	default:
		return "<unknown>"
	}
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

// SendByPhone
func (tx *Tx) SendByPhone() bool {
	return tx.ToPhone != nil && tx.ToWalletID == nil
}

// SendByWallet
func (tx *Tx) SendByWallet() bool {
	return tx.ToWalletID != nil && tx.ToPhone == nil
}

// SendByWallet
func (tx *Tx) SendByAddress() bool {
	return tx.ToAddress != nil
}

// IsSelfTx
func (tx *Tx) IsSelfTx() bool {
	selfTxByWallet := tx.ToWalletID != nil && tx.FromWalletID == *tx.ToWalletID
	selfTxByPhone := tx.ToPhone != nil && tx.FromWallet.UserPhone == *tx.ToPhone
	return selfTxByPhone || selfTxByWallet
}

// ExternalTx represents external transaction row
type TxExternal struct {
	ID        int64
	TxID      int64
	Tx        *Tx `gorm:"foreignkey:TxID;association_autocreate:false"`
	Hash      string
	Recipient string
}

func (TxExternal) TableName() string {
	return "txs_external"
}
