package nodes

import (
	"context"
)

// IGenerator used to generate user wallet for specific coin
type IGenerator interface {
	// Create coin wallet address
	Create(ctx context.Context) (address string, err error)
}

// retErrGenerator returns error on each call
type retErrGenerator struct {
	e error
}

// GetBalance implements IAccountObserver
func (obs retErrGenerator) Create(ctx context.Context) (address string, err error) {
	return "", obs.e
}
