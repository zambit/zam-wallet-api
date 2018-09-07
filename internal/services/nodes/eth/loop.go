package eth

import (
	"context"
	"time"
)

const ethPoolTimeout = time.Duration(float64(time.Minute) * 3.5)

// Run
func (node *ethNode) Run(ctx context.Context) error {

	l := node.logger.WithField("module", "nodes.eth.watcher_loop")

	// make greetings
	l.Info("starting watcher loop")

	// int each iteration, after sleep, we request block info
	// remember last block hash to eliminate subscriber calls when block not actually changed
	var lastBlockIndex int
	for {
		l.Debug("getting best block info")
		currentBestBlockIndex, err := node.getBestBlockIndex(ctx)
		if err != nil {
			l.WithError(err).Error("error getting best block index")
		}
		l.WithField("current_best_block", currentBestBlockIndex).Debug(currentBestBlockIndex)

		// call callback
		if lastBlockIndex != currentBestBlockIndex {
			sErr := node.subscriber(ctx, currentBestBlockIndex)
			if sErr != nil {
				l.WithError(sErr).Error("subscriber returns error")
			}
		}

		sleepDuration := ethPoolTimeout
		l.WithField("sleep_duration", sleepDuration).Info("sleeping")

		select {
		case <-ctx.Done():
			l.Info("stopping loop due to cancellation")
			return nil
		case <-time.After(sleepDuration):
		}
	}
}

// OnNewBlockReleased
func (node *ethNode) OnNewBlockReleased(f func(ctx context.Context, blockHeight int) error) {
	node.subscriber = f
}
