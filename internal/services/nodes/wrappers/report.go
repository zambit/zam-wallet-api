package wrappers

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/web-api/pkg/services/sentry"
	"github.com/ericlagergren/decimal"
)

// NewCoordinatorWrapper
func NewCoordinatorWrapper(c nodes.ICoordinator, reporter sentry.IReporter) nodes.ICoordinator {
	return &coordinatorMultiWrapper{reporter: reporter, coordinator: c}
}

// coordinatorMultiWrapper
type coordinatorMultiWrapper struct {
	reporter    sentry.IReporter
	coordinator nodes.ICoordinator
}

func (c *coordinatorMultiWrapper) checkErr(err error, coin string) {
	if err != nil {
		c.reporter.ReportErr(err, map[string]string{"coin": coin})
	}
}

func (c *coordinatorMultiWrapper) safeInvoke(coin string, m func() error) error {
	var err error
	c.reporter.CapturePanic(func() {
		err = m()
	}, map[string]string{"coin": coin})
	c.checkErr(err, coin)
	return err
}

func (c *coordinatorMultiWrapper) Dial(
	coinName string,
	host, user, pass string,
	testnet bool,
	additionalParams map[string]interface{},
) error {
	return c.safeInvoke(coinName, func() error {
		return c.coordinator.Dial(coinName, host, user, pass, testnet, additionalParams)
	})
}

func (c *coordinatorMultiWrapper) Close() error {
	return c.safeInvoke("", func() error {
		return c.Close()
	})
}

func (c *coordinatorMultiWrapper) WatcherLoop(coinName string) (nodes.IWatcherLoop, error) {
	return c.coordinator.WatcherLoop(coinName)
}

func (c *coordinatorMultiWrapper) Generator(coinName string) nodes.IGenerator {
	return &multiWrapper{IGenerator: c.coordinator.Generator(coinName), coin: coinName, reporter: c.reporter}
}

func (c *coordinatorMultiWrapper) Observer(coinName string) nodes.IWalletObserver {
	return &multiWrapper{IWalletObserver: c.coordinator.Observer(coinName), coin: coinName, reporter: c.reporter}
}

func (c *coordinatorMultiWrapper) AccountObserver(coinName string) nodes.IAccountObserver {
	return &multiWrapper{IAccountObserver: c.coordinator.AccountObserver(coinName), coin: coinName, reporter: c.reporter}
}

func (c *coordinatorMultiWrapper) TxsObserver(coinName string) nodes.ITxsObserver {
	return &multiWrapper{ITxsObserver: c.coordinator.TxsObserver(coinName), coin: coinName, reporter: c.reporter}
}

func (c *coordinatorMultiWrapper) TxsSender(coinName string) nodes.ITxSender {
	return &multiWrapper{ITxSender: c.coordinator.TxsSender(coinName), coin: coinName, reporter: c.reporter}
}

// reportWrapper
type multiWrapper struct {
	reporter sentry.IReporter
	coin     string

	nodes.IAccountObserver
	nodes.IWalletObserver
	nodes.IGenerator
	nodes.ITxSender
	nodes.ITxsObserver
	nodes.IWatcherLoop
}

func (w *multiWrapper) getTags() map[string]string {
	return map[string]string{
		"coin": w.coin,
	}
}

func (w *multiWrapper) checkErr(err error) {
	if err != nil {
		w.reporter.ReportErr(err, w.getTags())
	}
}

func (w *multiWrapper) safeInvoke(m func() error) {
	var err error
	w.reporter.CapturePanic(func() {
		err = m()
	}, w.getTags())
	w.checkErr(err)
}

func (w *multiWrapper) GetBalance(ctx context.Context) (balance *decimal.Big, err error) {
	w.safeInvoke(func() error {
		balance, err = w.IAccountObserver.GetBalance(ctx)
		return err
	})
	return
}

func (w *multiWrapper) Balance(ctx context.Context, address string) (balance *decimal.Big, err error) {
	w.safeInvoke(func() error {
		balance, err = w.IWalletObserver.Balance(ctx, address)
		return err
	})
	return
}

func (w *multiWrapper) Create(ctx context.Context) (address string, secret string, err error) {
	w.safeInvoke(func() error {
		address, secret, err = w.IGenerator.Create(ctx)
		return err
	})
	return
}

func (w *multiWrapper) SupportInternalTxs() bool {
	return w.ITxSender.SupportInternalTxs()
}

func (w *multiWrapper) Send(
	ctx context.Context,
	fromAddress, toAddress string,
	amount *decimal.Big,
) (txHash string, fee *decimal.Big, err error) {
	w.safeInvoke(func() error {
		txHash, fee, err = w.ITxSender.Send(ctx, fromAddress, toAddress, amount)
		return err
	})
	return
}

func (w *multiWrapper) IsConfirmed(ctx context.Context, hash string) (confirmed, abandoned bool, err error) {
	w.safeInvoke(func() error {
		confirmed, abandoned, err = w.ITxsObserver.IsConfirmed(ctx, hash)
		return err
	})
	return
}

func (w *multiWrapper) GetIncoming(ctx context.Context) (txs []nodes.IncomingTxDescr, err error) {
	w.safeInvoke(func() error {
		txs, err = w.ITxsObserver.GetIncoming(ctx)
		return err
	})
	return
}
