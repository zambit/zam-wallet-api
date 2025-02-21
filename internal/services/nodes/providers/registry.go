package providers

import (
	"github.com/sirupsen/logrus"
	"io"
	"strings"
)

// Provider
type Provider interface {
	Dial(
		logger logrus.FieldLogger,
		host, user, pass string,
		testnet bool,
		additionalParams map[string]interface{},
	) (io.Closer, error)
}

// registry is registry of per-coin service Provider
var registry map[string]Provider

// Register Provider for a specific coin, must be called before app main
func Register(coinName string, p Provider) {
	coinName = strings.ToUpper(coinName)
	logrus.Info("Register coin " + coinName)
	if registry == nil {
		registry = make(map[string]Provider)
	}

	registry[coinName] = p
}

// Get provider for coin
func Get(coinName string) (Provider, bool) {
	coinName = strings.ToUpper(coinName)
	logrus.Info("GET coin " + coinName)
	p, ok := registry[coinName]
	return p, ok
}
