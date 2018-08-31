package nodes

import (
	"context"
	"github.com/ericlagergren/decimal"
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

// ITxSender sends transaction from specified address
type ITxSender interface {
	// Send transaction from address to address with given amount in default coin units (BTC, ETH as example), returns
	// new transaction hash. If any of addresses is invalid, returns ErrAddressInvalid.
	// TODO interface must be extended in order to configure fee deducing strategy and fee amount.
	Send(ctx context.Context, fromAddress, toAddress string, amount *decimal.Big) (txHash string, err error)
}

// retErrTxs returns error on each call
type retErrTxs struct {
	e error
}

// GetHeight implements ITxsObserver
func (r retErrTxs) IsConfirmed(ctx context.Context, hash string) (confirmed bool, err error) {
	return false, r.e
}

// Send implements ITxSender
func (r retErrTxs) Send(ctx context.Context, fromAddress, toAddress string, amount *decimal.Big) (txHash string, err error) {
	return "", r.e
}
