package providers

import (
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert/icex"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
)

// CryptoCurrency create icex coin converter with default host
func CoinConverter() (c convert.ICryptoCurrency, err error) {
	defer func() {
		r := recover()
		if r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()

	c = icex.New("https://api.icex.ch")
	return
}
