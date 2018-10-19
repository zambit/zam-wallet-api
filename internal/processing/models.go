package processing

import (
	"database/sql/driver"
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
	TxStateExternalSending    = "send_external"
	TxStateDeclined           = "decline"
	TxStateCanceled           = "cancel"
	TxStateAwaitRecipient     = "pending"
	TxStateAwaitConfirmations = "waiting"
	TxStateProcessed          = "success"
)

// Decimal is a PostgreSQL DECIMAL. Its zero value is valid for use with both
// Value and Scan.
type Decimal postgres.Decimal

// Value implements driver.Valuer.
func (d *Decimal) Value() (driver.Value, error) {
	if d == nil {
		return nil, nil
	}
	return (*postgres.Decimal)(d).Value()
}

// Scan implements sql.Scanner
func (d *Decimal) Scan(src interface{}) error {
	if d == nil {
		return nil
	}
	return (*postgres.Decimal)(d).Scan(src)
}

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

	BlockchainFee *Decimal
	Amount        *Decimal

	Secret string

	CreatedAt time.Time
	UpdatedAt time.Time

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
	} else if tx.ToWallet != nil {
		return tx.ToWallet.Coin.ShortName
	}
	return "<unknown>"
}

// IsExternal
func (tx *Tx) IsExternal() bool {
	return tx.Type == TxTypeExternal
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
	return tx.ToWalletID != nil
}

// SendByWallet
func (tx *Tx) SendByAddress() bool {
	return tx.ToAddress != nil
}

// IsOutgoingForSide decides is this transaction generated by user with specified phone number. Phone number is
// must be normalized. This field is required because this model - is both side representation.
func (tx *Tx) IsOutgoingForSide(phone string) bool {
	// external transaction w/o from wallet field is incoming
	if tx.Type == TxTypeExternal && tx.FromWalletID == 0 {
		return false
	}
	return tx.FromWallet.UserPhone == phone
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
	Tx        *Tx `gorm:"foreignkey:TxID;association_autocreate:false;association_autoupdate:false"`
	Hash      string
	Recipient string
}

func (TxExternal) TableName() string {
	return "txs_external"
}
