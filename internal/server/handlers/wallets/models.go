package wallets

import (
	decimal2 "git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"strconv"
	"strings"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"github.com/ericlagergren/decimal"
)

// CreateRequest used to parse create wallet request body params
type CreateRequest struct {
	Coin       string `json:"coin"`
	WalletName string `json:"wallet_name" validate:"omitempty,min=3"`
}

// GetRequest used to bind query params to get wallet by id request
type GetRequest struct {
	Convert string `form:"convert"`
}

func DefaultGetRequest() GetRequest {
	return GetRequest{
		Convert: "usd",
	}
}

// GetAllRequest used to parse get all wallets request filter parms bounded from query
type GetAllRequest struct {
	ByCoin  string `form:"coin"`
	Cursor  string `form:"cursor"`
	Count   int64  `form:"count"`
	Convert string `form:"convert"`
}

func DefaultGetAllRequest() GetAllRequest {
	return GetAllRequest{
		Count:   10,
		Convert: "usd",
	}
}

// View used to represent wallet model
type View struct {
	ID       string                    `json:"id"`
	Coin     string                    `json:"coin"`
	Name     string                    `json:"wallet_name"`
	Address  string                    `json:"address"`
	Balances map[string]*decimal2.View `json:"balances"`
}

// AdditionalRate used to convert crypto-currency balance into additional currency
type AdditionalRate struct {
	*convert.Rate
	Currency string
}

// AdditionalRates same as AdditionalRate, but for multiple crypto-currency balances
type AdditionalRates struct {
	convert.MultiRate
	Currency string
}

// Response represents create and get wallets response
type Response struct {
	Wallet View `json:"wallet"`
}

// AllResponse represents get all wallets response
type AllResponse struct {
	Count   int64  `json:"count"`
	Next    string `json:"next"`
	Wallets []View `json:"wallets"`
}

var zeroDecimalView = (*decimal2.View)(new(decimal.Big).SetFloat64(0))

// ResponseFromWallet renders wallet view converting wallet id into string, also uses additional balances mapping
func ResponseFromWallet(wallet wallets.WalletWithBalance, additionalRate AdditionalRate) Response {
	balances := map[string]*decimal2.View{
		strings.ToLower(wallet.Coin.ShortName): (*decimal2.View)(wallet.Balance),
	}
	if additionalRate.Currency != ""  {
		var value *decimal2.View
		if additionalRate.Rate != nil {
			value = (*decimal2.View)(additionalRate.Convert(wallet.Balance))
		} else {
			value = zeroDecimalView
		}
		balances[additionalRate.Currency] = value
	}

	return Response{
		Wallet: View{
			ID:       getWalletIDView(wallet.ID),
			Coin:     strings.ToLower(wallet.Coin.ShortName),
			Name:     wallet.Name,
			Address:  wallet.Address,
			Balances: balances,
		},
	}
}

// AllResponseFromWallets prepares wallets representation
func AllResponseFromWallets(
	wallets []wallets.WalletWithBalance,
	totalCount int64,
	hasNext bool,
	additionalRates AdditionalRates,
) AllResponse {
	views := make([]View, 0, len(wallets))
	var next string
	if len(wallets) > 0 && hasNext {
		next = getWalletIDView(wallets[len(wallets)-1].ID)
	}

	for _, w := range wallets {
		views = append(views, ResponseFromWallet(
			w,
			AdditionalRate{
				additionalRates.CurrencyRate(w.Coin.ShortName),
				additionalRates.Currency,
			},
		).Wallet)
	}
	return AllResponse{
		Count:   totalCount,
		Next:    next,
		Wallets: views,
	}
}

// getWalletIDView wallet id to view representation
func getWalletIDView(id int64) string {
	return strconv.FormatInt(id, 10)
}

// parseWalletIDView
func parseWalletIDView(rawID string) (id int64, valid bool) {
	id, parseIntErr := strconv.ParseInt(rawID, 10, 64)
	valid = parseIntErr == nil
	return
}
