package providers

import (
	"io"
	"strings"
)

// Provider is not typo, it's noun derived from verb Dial
type Provider interface {
	Dial(host, user, pass string, testnet bool) (io.Closer, error)
}

// registry is registry of per-coin service Provider
var registry map[string]Provider

// Register Provider for a specific coin, must be called before app main
func Register(coinName string, p Provider) {
	coinName = strings.ToUpper(coinName)
	if registry == nil {
		registry = make(map[string]Provider)
	}

	registry[coinName] = p
}

// Get provider for coin
func Get(coinName string) (Provider, bool) {
	coinName = strings.ToUpper(coinName)
	p, ok := registry[coinName]
	return p, ok
}
