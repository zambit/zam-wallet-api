package nodes

import "context"

// IWatcherLoop is the event loop, used to perform actions when some events occurred in block-chain
type IWatcherLoop interface {
	// Run blocking event-loop, uses given context done chanel to stop the loop
	Run(ctx context.Context) error

	// OnNewBlockReleased perform subscriber each time new block is attached to block-chain. Subscriber may return error
	// which will be logged. May skip callback if the last call took to long.
	//
	// Subscriber must stop processing when context done channel is closed
	OnNewBlockReleased(func(ctx context.Context, blockHeight int) error)
}
