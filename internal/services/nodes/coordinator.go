package nodes

import (
	"errors"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes/providers"
	"github.com/sirupsen/logrus"
	"io"
	"strings"
)

var (
	ErrNoSuchCoin = errors.New("coordinator: no such coin found")

	ErrCoinIsUnsupported = errors.New("coordinator: coin is unsupported")

	ErrCoinServiceNotImplemented = errors.New("coordinator: coin service is not implemented")
)

// ICoordinator coordinates nodes rpc wrappers which split into different interfaces which may be accessed usign special
// calls such as Generator, Observer etc. It panics with ErrNoSuchCoin if no such coin registered. Also this methods
// is case insensitive.
type ICoordinator interface {
	// Dial coin for given params and add coin services to this coordinator
	//
	// If there is no actual implementation for required coin, ErrCoinIsUnsupported will be returned.
	Dial(coinName string, host, user, pass string, testnet bool, additionalParams map[string]interface{}) error

	// Close closes all connections
	Close() error

	// WatcherLoop returns watcher loop implementation for specified coin or ErrNoSuchCoin.
	WatcherLoop(coinName string) (IWatcherLoop, error)

	// Generator returns generator which belongs to a specified coin.
	Generator(coinName string) IGenerator

	// Observer returns wallet observer for specified coin.
	Observer(coinName string) IWalletObserver

	// AccountObserver returns account observer for specific coin.
	AccountObserver(coinName string) IAccountObserver

	// TxsObserver get txs observer implementation by coin name
	TxsObserver(coinName string) ITxsObserver

	// TxsSender get tx sender implementation by coin name
	TxsSender(coinName string) ITxSender
}

// New creates new default coordinator
func New(logger logrus.FieldLogger) ICoordinator {
	return &coordinator{
		logger:           logger.WithField("module", "wallets.coordinator"),
		closers:          make(map[string]io.Closer),
		generators:       make(map[string]IGenerator),
		observers:        make(map[string]IWalletObserver),
		accountObservers: make(map[string]IAccountObserver),
		txsObserevers:    make(map[string]ITxsObserver),
		watchers:         make(map[string]IWatcherLoop),
		senders:          make(map[string]ITxSender),
	}
}

// coordinator implements ICoordinator in straight way
type coordinator struct {
	logger           logrus.FieldLogger
	closers          map[string]io.Closer
	generators       map[string]IGenerator
	observers        map[string]IWalletObserver
	accountObservers map[string]IAccountObserver
	txsObserevers    map[string]ITxsObserver
	watchers         map[string]IWatcherLoop
	senders          map[string]ITxSender
}

// Dial lookup service provider registry, dial no safe with concurrent getters usage
func (c *coordinator) Dial(
	coinName string, host, user, pass string, testnet bool,
	additionalParams map[string]interface{},
) error {
	coinName = strings.ToUpper(coinName)

	provider, ok := providers.Get(coinName)
	if !ok {
		return ErrCoinIsUnsupported
	}

	services, err := provider.Dial(c.logger, host, user, pass, testnet, additionalParams)
	if err != nil {
		return err
	}

	c.closers[coinName] = services

	if generator, ok := services.(IGenerator); ok {
		c.generators[coinName] = generator
	}

	if observer, ok := services.(IWalletObserver); ok {
		c.observers[coinName] = observer
	}

	if observer, ok := services.(IAccountObserver); ok {
		c.accountObservers[coinName] = observer
	}

	if observer, ok := services.(ITxsObserver); ok {
		c.txsObserevers[coinName] = observer
	}

	if loop, ok := services.(IWatcherLoop); ok {
		c.watchers[coinName] = loop
	}

	if sender, ok := services.(ITxSender); ok {
		c.senders[coinName] = sender
	}

	return nil
}

// Close implements ICoordinator interface
func (c *coordinator) Close() (err error) {
	for _, closer := range c.closers {
		cErr := closer.Close()
		if cErr != nil {
			err = merrors.Append(err, cErr)
		}
	}

	return
}

// Generator implements ICoordinator interface
func (c *coordinator) Generator(coinName string) IGenerator {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	generator, ok := c.generators[coinName]
	if !ok {
		return retErrGenerator{e: ErrCoinServiceNotImplemented}
	}

	return generator
}

// Observer implements ICoordinator interface
func (c *coordinator) Observer(coinName string) IWalletObserver {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	observer, ok := c.observers[coinName]
	if !ok {
		return retErrWalletObserver{e: ErrCoinServiceNotImplemented}
	}
	return observer
}

// AccountObserver implements ICoordinator interface
func (c *coordinator) AccountObserver(coinName string) IAccountObserver {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	observer, ok := c.accountObservers[coinName]
	if !ok {
		return retErrAccountObserver{e: ErrCoinServiceNotImplemented}
	}
	return observer
}

// TxsObserver implements ICoordinator interface
func (c *coordinator) TxsObserver(coinName string) ITxsObserver {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	observer, ok := c.txsObserevers[coinName]
	if !ok {
		return retErrTxs{e: ErrCoinServiceNotImplemented}
	}
	return observer
}

// TxsObserver implements ICoordinator interface
func (c *coordinator) WatcherLoop(coinName string) (IWatcherLoop, error) {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		return nil, ErrNoSuchCoin
	}

	observer, ok := c.watchers[coinName]
	if !ok {
		return nil, ErrCoinServiceNotImplemented
	}
	return observer, nil
}

// TxsSender implements ICoordinator interface
func (c *coordinator) TxsSender(coinName string) ITxSender {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	sender, ok := c.senders[coinName]
	if !ok {
		return retErrTxs{e: ErrCoinServiceNotImplemented}
	}
	return sender
}
