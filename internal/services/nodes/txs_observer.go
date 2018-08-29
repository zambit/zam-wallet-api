package nodes

import (
	"context"
	"github.com/pkg/errors"
)

var (
	// ErrNoSuchTx indicates that no tx found which satisfies given conditions
	ErrNoSuchTx = errors.New("txs observer: no such tx")
)

// ITxsObserver used to observe transaction usually by their hash
type ITxsObserver interface {
	// IsConfirmed query tx confirmations count by tx hash and decides if tx confirmed or not, returns ErrNoSuchTx if
	// tx hasn't been found.
	IsConfirmed(ctx context.Context, hash string) (confirmed bool, err error)
}

// retErrTxsObserver returns error on each call
type retErrTxsObserver struct {
	e error
}

// GetHeight implements ITxsObserver
func (r retErrTxsObserver) IsConfirmed(ctx context.Context, hash string) (confirmed bool, err error) {
	return false, r.e
}
