package txs

import (
	"git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/wallets"
	"strconv"
	"strings"
	"time"
)

// SendRequest used to parse send tx request body
type SendRequest struct {
	WalletID  int64         `json:"wallet_id,string" validate:"required"`
	Recipient string        `json:"recipient" validate:"required,phone"`
	Amount    *decimal.View `json:"amount" validate:"required"`
}

// GetAllRequest get all wallets request query params parser
type GetAllRequest struct {
	Coin      *string `form:"coin"`
	Status    *string `form:"status"`
	WalletID  *string `form:"wallet_id"`
	Recipient *string `form:"recipient"`
	FromTime  *string `form:"from_time"`
	UntilTime *string `form:"until_time"`
	Page      *string `form:"page"`
	Count     *int64  `form:"count"`
}

// UnixTimeView marshales into unix time format
type UnixTimeView time.Time

func (t *UnixTimeView) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt((*time.Time)(t).Unix(), 10)), nil
}

// View tx api representation
type View struct {
	ID        string        `json:"id"`
	WalletID  string        `json:"wallet_id,omitempty"`
	Direction string        `json:"direction"`
	Status    string        `json:"status"`
	Coin      string        `json:"coin"`
	Recipient string        `json:"recipient,omitempty"`
	Sender    string        `json:"sender,omitempty"`
	Amount    *decimal.View `json:"amount"`
	CreatedAt UnixTimeView  `json:"created_at"`
}

// SingleResponse single tx response
type SingleResponse struct {
	Transaction *View `json:"transaction"`
}

// MultipleResponse
type MultipleResponse struct {
	Count        int64  `json:"count"`
	Next         string `json:"next"`
	Transactions []View `json:"transactions"`
}

// ToIdView converts tx id to api representation
func ToIdView(id int64) string {
	return strconv.FormatInt(id, 10)
}

// FromIdView converts id api representation into tx is and provides valid flag
func FromIdView(idView string) (id int64, valid bool) {
	id, parseIntErr := strconv.ParseInt(idView, 10, 64)
	valid = parseIntErr == nil
	return
}

// ToView
func ToView(tx *processing.Tx, userPhone string) *View {
	isOutgoing := tx.FromWallet.UserPhone == userPhone
	// wallet id must be shadowed if tx is incoming
	var (
		walletID  string
		recipient string
		sender    string
	)
	if isOutgoing {
		walletID = wallets.GetWalletIDView(tx.FromWalletID)
		if tx.ToPhone != nil {
			recipient = *tx.ToPhone
		} else {
			recipient = tx.ToWallet.UserPhone
		}
	} else {
		sender = tx.FromWallet.UserPhone
	}

	return &View{
		ID:        ToIdView(tx.ID),
		WalletID:  walletID,
		Direction: map[bool]string{true: "outgoing", false: "incoming"}[isOutgoing],
		Status:    tx.Status.Name,
		Coin:      strings.ToLower(tx.FromWallet.Coin.ShortName),
		Recipient: recipient,
		Sender:    sender,
		Amount:    (*decimal.View)(tx.Amount.V),
		CreatedAt: UnixTimeView(tx.CreatedAt),
	}
}

// ToAllView
func ToAllView(txs []processing.Tx, userPhone string) []View {
	res := make([]View, len(txs))
	for i, tx := range txs {
		// ToView return should not escape
		res[i] = *ToView(&tx, userPhone)
	}
	return res
}
