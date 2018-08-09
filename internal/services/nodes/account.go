package nodes

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/ctxext"
	"github.com/ericlagergren/decimal"
)

// IAccountObserver observes whole node account, not a specific address as IWalletObserver
type IAccountObserver interface {
	ctxext.ContextAttacher

	// GetBalance get node account balance
	GetBalance() (balance *decimal.Big, err error)
}

// retErrAccountObserver returns error on each call
type retErrAccountObserver struct {
	e error
}

// WithContext implements IAccountObserver
func (obs retErrAccountObserver) WithContext(ctx context.Context) interface{} {
	return obs
}

// GetBalance implements IAccountObserver
func (obs retErrAccountObserver) GetBalance() (balance *decimal.Big, err error) {
	return nil, obs.e
}
