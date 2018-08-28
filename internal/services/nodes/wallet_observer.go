package nodes

import (
	"context"
	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
)

var (
	// ErrAddressInvalid returned when address can't be observed because not found or invalid
	ErrAddressInvalid = errors.New("observer: address invalid")
)

// IWalletObserver observes wallets state
type IWalletObserver interface {
	// Balances returns actual address balance
	Balance(ctx context.Context, address string) (*decimal.Big, error)
}

// retErrAccountObserver returns error on each call
type retErrWalletObserver struct {
	e error
}

// GetBalance implements IAccountObserver
func (obs retErrWalletObserver) Balance(ctx context.Context, address string) (balance *decimal.Big, err error) {
	return nil, obs.e
}
