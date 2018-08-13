package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers/icex"
)

// CoinConverter create icex coin converter with default host
func CoinConverter() (c helpers.ICoinConverter, err error) {
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
