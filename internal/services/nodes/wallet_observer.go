package nodes

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/ctxext"
	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
)

var (
	// ErrAddressInvalid returned when address can't be observed because not found or invalid
	ErrAddressInvalid = errors.New("observer: address invalid")
)

// IWalletObserver observes wallets state
type IWalletObserver interface {
	ctxext.ContextAttacher

	// Balance returns actual address balance
	Balance(address string) (*decimal.Big, error)
}

// retErrAccountObserver returns error on each call
type retErrWalletObserver struct {
	e error
}

// WithContext implements IAccountObserver
func (obs retErrWalletObserver) WithContext(ctx context.Context) interface{} {
	return obs
}

// GetBalance implements IAccountObserver
func (obs retErrWalletObserver) Balance(address string) (balance *decimal.Big, err error) {
	return nil, obs.e
}
