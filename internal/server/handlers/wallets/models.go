package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/common"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"strconv"
	"strings"
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

// GetAllRequest used to parse get all wallets request filter parms bounded from query
type GetAllRequest struct {
	ByCoin  string `form:"coin"`
	Cursor  string `form:"cursor"`
	Count   int64  `form:"count"`
	Convert string `form:"convert"`
}

// View used to represent wallet model
type View struct {
	ID       string                      `json:"id"`
	Coin     string                      `json:"coin"`
	Name     string                      `json:"wallet_name"`
	Address  string                      `json:"address"`
	Balances common.MultiCurrencyBalance `json:"balances"`
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

// ResponseFromWallet renders wallet view converting wallet id into string, also uses additional balances mapping
func ResponseFromWallet(wallet wallets.WalletWithBalance, additionalRate common.AdditionalRate) Response {
	additionalRate.CoinCurrency = wallet.Coin.ShortName
	return Response{
		Wallet: View{
			ID:       GetWalletIDView(wallet.ID),
			Coin:     strings.ToLower(wallet.Coin.ShortName),
			Name:     wallet.Name,
			Address:  wallet.Address,
			Balances: additionalRate.RepresentBalance(wallet.Balance),
		},
	}
}

// AllResponseFromWallets prepares wallets representation
func AllResponseFromWallets(
	wallets []wallets.WalletWithBalance,
	totalCount int64,
	hasNext bool,
	additionalRates common.AdditionalRates,
) AllResponse {
	views := make([]View, 0, len(wallets))
	var next string
	if len(wallets) > 0 && hasNext {
		next = GetWalletIDView(wallets[len(wallets)-1].ID)
	}

	for _, w := range wallets {
		views = append(views, ResponseFromWallet(w, additionalRates.ForCoinCurrency(w.Coin.ShortName)).Wallet)
	}
	return AllResponse{
		Count:   totalCount,
		Next:    next,
		Wallets: views,
	}
}

// GetWalletIDView wallet id to view representation
func GetWalletIDView(id int64) string {
	return strconv.FormatInt(id, 10)
}

// ParseWalletIDView
func ParseWalletIDView(rawID string) (id int64, valid bool) {
	id, parseIntErr := strconv.ParseInt(rawID, 10, 64)
	valid = parseIntErr == nil
	return
}
