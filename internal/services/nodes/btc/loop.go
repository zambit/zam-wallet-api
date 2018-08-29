package btc

import (
	"context"
	"time"
)

const (
	btcDefaultTimeBetweenBlocks = time.Minute * 9
	minimalSleepTime            = time.Minute
)

// Run
func (n *btcNode) Run(ctx context.Context) error {
	l := n.logger.WithField("module", "nodes."+n.coinName+".watcher_loop")

	// make greetings
	l.WithField("coin", n.coinName).Info("starting watcher loop")

	// int each iteration, after sleep, we request block info
	// remember last block hash to eliminate subscriber calls when block not actually changed
	var lastBlockHash string
	for {
		l.Info("getting best block info")
		lastHash, lastHeigh, lastBlockTime, err := n.getBestBlockInfo()

		var sleepDuration time.Duration
		if err != nil {
			// if error occurs while getting node info, sleep for default time
			l.WithError(err).Error("getting of best block has been failed")
			sleepDuration = btcDefaultTimeBetweenBlocks
		} else {
			sinceLastBlock := time.Now().UTC().Sub(lastBlockTime)
			l.WithField(
				"last_hash", lastHash,
			).WithField(
				"last_block_time", lastBlockTime,
			).WithField(
				"since_last_block", sinceLastBlock,
			).Info("info got successfully")

			if sinceLastBlock > btcDefaultTimeBetweenBlocks {
				sleepDuration = btcDefaultTimeBetweenBlocks*2 - sinceLastBlock
			} else {
				sleepDuration = btcDefaultTimeBetweenBlocks - sinceLastBlock
			}
			// prevent short sleeps
			if sleepDuration < minimalSleepTime {
				sleepDuration = minimalSleepTime
			}

			if lastHash != lastBlockHash {
				l.WithField("last_height", lastHeigh).Info("new block released")

				cCtx, _ := context.WithTimeout(ctx, sleepDuration)
				err := n.subscriber(cCtx, int(lastHeigh))
				if err != nil {
					l.WithError(err).Error("error occurs while processing subscriber")
				}
			}
		}

		l.WithField("sleep_duration", sleepDuration).Info("sleeping")

		select {
		case <-ctx.Done():
			l.Info("stopping loop due to cancellation")
			return nil
		case <-time.After(sleepDuration):
		}
	}
}

// OnNewBlockReleased implements IWatcherLoop interface
func (n *btcNode) OnNewBlockReleased(subscriber func(ctx context.Context, blockHeight int) error) {
	n.subscriber = subscriber
}

func (n *btcNode) getBestBlockInfo() (lastHash string, lastHeigh int64, lastBlockTime time.Time, err error) {
	err = n.doCall("getbestblockhash", &lastHash)
	if err != nil {
		return
	}

	var bestBlock struct {
		MedianTime int64 `json:"mediantime"`
		Height     int64 `json:"height"`
	}
	err = n.doCall("getblock", &bestBlock, lastHash)
	if err != nil {
		return
	}

	lastBlockTime = time.Unix(bestBlock.MedianTime, 0).UTC()
	lastHeigh = bestBlock.Height
	return
}
