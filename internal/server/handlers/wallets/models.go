package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"github.com/ericlagergren/decimal"
	"strconv"
	"strings"
)

// CreateRequest used to parse create wallet request body params
type CreateRequest struct {
	Coin       string `json:"coin"`
	WalletName string `json:"wallet_name" validate:"omitempty,min=3"`
}

// GetAllRequest used to parse get all wallets request filter parms bounded from query
type GetAllRequest struct {
	ByCoin string `form:"coin"`
	Cursor string `form:"cursor"`
	Count  int64  `form:"count"`
}

func DefaultGetAllRequest() GetAllRequest {
	return GetAllRequest{
		Count: 10,
	}
}

// View used to represent wallet model
type View struct {
	ID      string       `json:"id"`
	Coin    string       `json:"name"`
	Name    string       `json:"wallet_name"`
	Address string       `json:"address"`
	Balance *decimal.Big `json:"balance"`
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

// ResponseFromWallet renders wallet view converting wallet id into string
func ResponseFromWallet(wallet wallets.WalletWithBalance) Response {
	return Response{
		Wallet: View{
			ID:      getWalletIDView(wallet.ID),
			Coin:    strings.ToLower(wallet.Coin.ShortName),
			Name:    wallet.Name,
			Address: wallet.Address,
			Balance: wallet.Balance,
		},
	}
}

// AllResponseFromWallets prepares wallets representation
func AllResponseFromWallets(wallets []wallets.WalletWithBalance, totalCount int64, hasNext bool) AllResponse {
	views := make([]View, 0, len(wallets))
	var next string
	if len(wallets) > 0 && hasNext {
		next = getWalletIDView(wallets[len(wallets)-1].ID)
	}

	for _, w := range wallets {
		views = append(views, ResponseFromWallet(w).Wallet)
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
