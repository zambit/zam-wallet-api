package txs

import (
	"git.zam.io/wallet-backend/common/pkg/types"
	"git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/common"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/wallets"
	bdecimal "github.com/ericlagergren/decimal"
	"github.com/jinzhu/now"
	"strconv"
	"strings"
	"time"
)

// SendRequest used to parse send tx request body
type SendRequest struct {
	WalletID  int64         `json:"wallet_id,string" validate:"required"`
	Recipient string        `json:"recipient" validate:"required"`
	Amount    *decimal.View `json:"amount" validate:"required"`
}

// ConvertParams used in send tx request to parse query params
type ConvertParams struct {
	Convert string `form:"convert"`
}

// GetAllRequest get all wallets request query params parser
type GetAllRequest struct {
	Coin      *string `form:"coin"`
	Status    *string `form:"status"`
	WalletID  *string `form:"wallet_id"`
	Recipient *string `form:"recipient"`
	FromTime  *string `form:"from_time"`
	UntilTime *string `form:"until_time"`
	Direction *string `form:"direction"`
	Page      *string `form:"page"`
	Count     *int64  `form:"count"`
	Convert   string  `form:"convert"`
	Group     string  `form:"group"`
}

// View tx api representation
type View struct {
	ID        string                      `json:"id"`
	WalletID  string                      `json:"wallet_id"`
	Direction string                      `json:"direction"`
	Status    string                      `json:"status"`
	Coin      string                      `json:"coin"`
	Recipient string                      `json:"recipient,omitempty"`
	Sender    string                      `json:"sender,omitempty"`
	Type      string                      `json:"type"`
	Amount    common.MultiCurrencyBalance `json:"amount"`
	Fee       common.MultiCurrencyBalance `json:"fee,omitempty"`
	CreatedAt types.UnixTimeView          `json:"created_at"`
}

// SingleResponse single tx response
type SingleResponse struct {
	Transaction *View `json:"transaction"`
}

// MultipleResponse
type MultipleResponse struct {
	TotalCount   int64   `json:"total_count"`
	Count        int     `json:"count"`
	Next         *string `json:"next"`
	Transactions []View  `json:"transactions"`
}

// GroupView
type GroupView struct {
	StartDate    types.UnixTimeView          `json:"start_date"`
	EndDate      types.UnixTimeView          `json:"end_date"`
	TotalAmount  common.MultiCurrencyBalance `json:"total_amount"`
	Transactions []View                      `json:"items"`
}

// GroupedResponse holds txs in grouped parts
type GroupedResponse struct {
	TotalCount          int64       `json:"total_count"`
	Count               int         `json:"count"`
	Next                *string     `json:"next"`
	GroupedTransactions []GroupView `json:"transactions"`
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
func ToView(tx *processing.Tx, userPhone string, rate common.AdditionalRate) *View {
	// wallet id must be shadowed if tx is incoming
	var (
		walletID  string
		recipient string
		sender    string
		direction string
		fee       common.MultiCurrencyBalance
	)
	if tx.IsOutgoingForSide(userPhone) {
		walletID = wallets.GetWalletIDView(tx.FromWalletID)
		switch {
		case tx.SendByPhone():
			recipient = *tx.ToPhone
		case tx.SendByWallet():
			recipient = tx.ToWallet.UserPhone
		case tx.SendByAddress():
			recipient = *tx.ToAddress
		}
		if tx.BlockchainFee != nil {
			fee = rate.RepresentBalance(tx.BlockchainFee.V)
		}

		direction = "outgoing"
	} else {
		if tx.IsExternal() {
			// leave this field empty for external txs because we don't track such info :(
			sender = ""
		} else {
			sender = tx.FromWallet.UserPhone
		}

		if tx.ToWalletID != nil {
			walletID = wallets.GetWalletIDView(*tx.ToWalletID)
		}

		direction = "incoming"
	}

	coinName := strings.ToLower(tx.CoinName())
	rate.CoinCurrency = coinName
	return &View{
		ID:        ToIdView(tx.ID),
		WalletID:  walletID,
		Direction: direction,
		Status:    tx.Status.Name,
		Coin:      coinName,
		Recipient: recipient,
		Sender:    sender,
		Type:      string(tx.Type),
		Amount:    rate.RepresentBalance(tx.Amount.V),
		Fee:       fee,
		CreatedAt: types.UnixTimeView(tx.CreatedAt),
	}
}

// ToGroupViews
func ToGroupViews(txs []processing.Tx, userPhone string, rates common.AdditionalRates, group string) []GroupView {
	if len(txs) == 0 {
		return nil
	}

	// TODO approximate groups count
	groups := make([]GroupView, 0, 10)

	// determine groups func on group arg
	groupStartFunc := func(time.Time) time.Time {
		return time.Time{}
	}
	groupEndFunc := func(time.Time) time.Time {
		return time.Time{}
	}

	switch group {
	case "hour":
		groupStartFunc = func(t time.Time) time.Time {
			return now.New(t).BeginningOfHour()
		}
		groupEndFunc = func(t time.Time) time.Time {
			return now.New(t).EndOfHour().Add(time.Nanosecond * 2)
		}
	case "day":
		groupStartFunc = func(t time.Time) time.Time {
			return now.New(t).BeginningOfDay()
		}
		groupEndFunc = func(t time.Time) time.Time {
			return now.New(t).EndOfDay().Add(time.Nanosecond * 2)
		}
	case "week":
		groupStartFunc = func(t time.Time) time.Time {
			return now.New(t).BeginningOfWeek()
		}
		groupEndFunc = func(t time.Time) time.Time {
			return now.New(t).EndOfWeek().Add(time.Nanosecond * 2)
		}
	case "month":
		groupStartFunc = func(t time.Time) time.Time {
			return now.New(t).BeginningOfMonth()
		}
		groupEndFunc = func(t time.Time) time.Time {
			return now.New(t).EndOfMonth().Add(time.Nanosecond * 2)
		}
	default:
		panic("unexpected group")
	}

	//
	defaultCurrencyRate := rates.ForCoinCurrency(common.DefaultCryptoCurrency)
	for i := 0; i < len(txs); {
		startG := groupStartFunc(txs[i].CreatedAt)
		endG := groupEndFunc(txs[i].CreatedAt)

		groupped := make([]View, 0, 3)
		groupFiatTotal := new(bdecimal.Big)

		for y, tx := range txs[i:] {
			// stop group if tx out of group bounds
			if tx.CreatedAt.After(endG) || tx.CreatedAt.Before(startG) {
				i += y
				break
			}
			groupped = append(
				groupped,
				*ToView(
					&tx,
					userPhone,
					rates.ForCoinCurrency(tx.CoinName()),
				),
			)

			// calc total fiat sum depending on tx direction
			// not counts txs which is can be spent againt
			if tx.IsHoldsAmount() {
				txFiatAmount := rates.CurrencyRate(tx.CoinName()).Convert(tx.Amount.V)
				if !tx.IsOutgoingForSide(userPhone) {
					groupFiatTotal.Add(groupFiatTotal, txFiatAmount)
				} else {
					groupFiatTotal.Sub(groupFiatTotal, txFiatAmount)
				}
			}

			// advance i onto last iteration
			// simply to break outer loop
			if len(txs)-i-y == 1 {
				i += y + 1
			}
		}

		// convert back from total in fiat into default currency
		groupDefaultCoinTotal := defaultCurrencyRate.ReverseConvert(groupFiatTotal)

		// crete group
		groups = append(groups, GroupView{
			StartDate:    types.UnixTimeView(startG),
			EndDate:      types.UnixTimeView(endG),
			TotalAmount:  defaultCurrencyRate.RepresentBalance(groupDefaultCoinTotal),
			Transactions: groupped,
		})
	}

	return groups
}

// ToAllView
func ToAllView(txs []processing.Tx, userPhone string, rates common.AdditionalRates) []View {
	res := make([]View, len(txs))
	for i, tx := range txs {
		// ToView return should not escape
		res[i] = *ToView(&tx, userPhone, rates.ForCoinCurrency(tx.CoinName()))
	}
	return res
}
