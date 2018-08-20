package providers

import (
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert/icex"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
)

// CryptoCurrency create icex coin converter with default host
func CoinConverter() (c convert.ICryptoCurrency, err error) {
	return icex.New("https://api.icex.ch")
}
