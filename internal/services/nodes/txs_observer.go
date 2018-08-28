package nodes

import (
	"github.com/pkg/errors"
	"context"
)

var (
	// ErrNoSuchTx indicates that no tx found which satisfies given conditions
	ErrNoSuchTx = errors.New("txs observer: no such tx")
)

// ITxsObserver used to observe transaction usually by their hash
type ITxsObserver interface {
	// GetHeight query tx height by it hash, returns ErrNoSuchTx if tx hasn't been found.
	GetHeight(ctx context.Context, hash string) (height int, err error)
}

// retErrTxsObserver returns error on each call
type retErrTxsObserver struct {
	e error
}

// GetHeight implements ITxsObserver
func (r retErrTxsObserver) GetHeight(ctx context.Context, hash string) (height int, err error) {
	return 0, r.e
}
