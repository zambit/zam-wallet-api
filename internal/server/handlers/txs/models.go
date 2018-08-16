package txs

import (
	"git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"strconv"
	"strings"
)

// SendRequest used to parse send tx request body
type SendRequest struct {
	WalletID  int64         `json:"wallet_id,string" validate:"required"`
	Recipient string        `json:"recipient" validate:"required,phone"`
	Amount    *decimal.View `json:"amount" validate:"required"`
}

// View tx api representation
type View struct {
	ID        string        `json:"id"`
	Direction string        `json:"direction"`
	Status    string        `json:"status"`
	Coin      string        `json:"coin"`
	Recipient string        `json:"recipient"`
	Amount    *decimal.View `json:"amount"`
}

// SendResponse send request response
type SendResponse struct {
	Transaction *View `json:"transaction"`
}

// ToIdView converts tx id to api representation
func ToIdView(id int64) string {
	return strconv.FormatInt(id, 10)
}

// ToView
func ToView(tx *processing.Tx, userPhone string) *View {
	var recipient string
	if tx.ToPhone != nil {
		recipient = *tx.ToPhone
	} else {
		recipient = tx.ToWallet.UserPhone
	}

	return &View{
		ID:        ToIdView(tx.ID),
		Direction: map[bool]string{true: "outgoing", false: "incoming"}[tx.FromWallet.UserPhone == userPhone],
		Status:    tx.Status.Name,
		Coin:      strings.ToLower(tx.FromWallet.Coin.ShortName),
		Recipient: recipient,
		Amount:    (*decimal.View)(tx.Amount.V),
	}
}
