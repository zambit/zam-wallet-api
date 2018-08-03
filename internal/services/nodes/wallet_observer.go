package nodes

import (
	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
)

var (
	// ErrAddressInvalid returned when address can't be observed because not found or invalid
	ErrAddressInvalid = errors.New("observer: address invalid")
)

// IWalletObserver observes wallets state
type IWalletObserver interface {
	// Balance returns actual address balance
	Balance(address string) (*decimal.Big, error)
}
