package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/models"
	"strconv"
)

// CreateRequest used to parse create wallet request body params
type CreateRequest struct {
	Coin       string `json:"coin"`
	WalletName string `json:"wallet_name" validate:"omitempty,min=3"`
}

// View used to represent wallet model
type View struct {
	ID      string `json:"id"`
	Coin    string `json:"coin"`
	Name    string `json:"wallet_name"`
	Address string `json:"address"`
}

// Response
type Response struct {
	Wallet View `json:"wallet"`
}

// AllResponse
type AllResponse struct {
	Count   int64  `json:"count"`
	Next    string `json:"next"`
	Wallets []View `json:"wallets"`
}

// ResponseFromWallet renders wallet view converting wallet id into string
func ResponseFromWallet(wallet models.Wallet) Response {
	return Response{
		Wallet: View{
			ID:      getWalletIDView(wallet.ID),
			Coin:    wallet.Coin.ShortName,
			Name:    wallet.Name,
			Address: wallet.Address,
		},
	}
}

// AllResponseFromWallets
func AllResponseFromWallets(wallets []models.Wallet, totalCount int64) AllResponse {
	views := make([]View, len(wallets))
	var next string
	if len(wallets) > 0 {
		next = getWalletIDView(wallets[len(wallets)-1].ID)
	}

	for i, w := range wallets {
		views[i] = ResponseFromWallet(w).Wallet
	}
	return AllResponse{
		Count:   totalCount,
		Next:    next,
		Wallets: views,
	}
}

func getWalletIDView(id int64) string {
	return strconv.FormatInt(id, 10)
}
