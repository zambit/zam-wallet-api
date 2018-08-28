package nodes

import (
	"context"
	"github.com/ericlagergren/decimal"
)

// IAccountObserver observes whole node account, not a specific address as IWalletObserver
type IAccountObserver interface {
	// GetBalance get node account balance
	GetBalance(ctx context.Context) (balance *decimal.Big, err error)
}

// retErrAccountObserver returns error on each call
type retErrAccountObserver struct {
	e error
}

// GetBalance implements IAccountObserver
func (obs retErrAccountObserver) GetBalance(ctx context.Context) (balance *decimal.Big, err error) {
	return nil, obs.e
}
