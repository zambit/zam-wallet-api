package nodes

import (
	"context"
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
	Dial(coinName string, host, user, pass string, testnet bool) error

	// Close closes all connections
	Close() error

	// Generator returns generator which belongs to a specified coin or ErrNoSuchCoin
	Generator(coinName string) (IGenerator, error)

	// Observer returns wallet observer for specified coin.
	Observer(coinName string) IWalletObserver

	// ObserverWithCtx same as Observer, but attaches given context
	ObserverWithCtx(ctx context.Context, coinName string) IWalletObserver

	// AccountObserver returns account observer for specific coin.
	AccountObserver(coinName string) IAccountObserver

	// AccountObserver same as AccountObserver, but attaches given context
	AccountObserverWithCtx(ctx context.Context, coinName string) IAccountObserver
}

// New creates new default coordinator
func New(logger logrus.FieldLogger) ICoordinator {
	return &coordinator{
		logger:           logger.WithField("module", "wallets.coordinator"),
		closers:          make(map[string]io.Closer),
		generators:       make(map[string]IGenerator),
		observers:        make(map[string]IWalletObserver),
		accountObservers: make(map[string]IAccountObserver),
	}
}

// coordinator implements ICoordinator in straight way
type coordinator struct {
	logger           logrus.FieldLogger
	closers          map[string]io.Closer
	generators       map[string]IGenerator
	observers        map[string]IWalletObserver
	accountObservers map[string]IAccountObserver
}

// Dial lookup service provider registry, dial no safe with concurrent getters usage
func (c *coordinator) Dial(coinName string, host, user, pass string, testnet bool) error {
	coinName = strings.ToUpper(coinName)

	provider, ok := providers.Get(coinName)
	if !ok {
		return ErrCoinIsUnsupported
	}

	services, err := provider.Dial(c.logger, host, user, pass, testnet)
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
func (c *coordinator) Generator(coinName string) (IGenerator, error) {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		return nil, ErrNoSuchCoin
	}

	generator, ok := c.generators[coinName]
	if !ok {
		return nil, ErrCoinServiceNotImplemented
	}

	return generator, nil
}

// Generator implements ICoordinator interface
func (c *coordinator) ObserverWithCtx(ctx context.Context, coinName string) IWalletObserver {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	observer, ok := c.observers[coinName]
	if !ok {
		return retErrWalletObserver{e: ErrCoinServiceNotImplemented}
	}
	return observer.WithContext(ctx).(IWalletObserver)
}

// Observer implements ICoordinator interface
func (c *coordinator) Observer(coinName string) IWalletObserver {
	return c.ObserverWithCtx(context.Background(), coinName)
}

// AccountObserver implements ICoordinator interface
func (c *coordinator) AccountObserver(coinName string) IAccountObserver {
	return c.AccountObserverWithCtx(context.Background(), coinName)
}

// AccountObserverWithCtx implements ICoordinator interface
func (c *coordinator) AccountObserverWithCtx(ctx context.Context, coinName string) IAccountObserver {
	coinName = strings.ToUpper(coinName)

	if _, ok := c.closers[coinName]; !ok {
		panic(ErrNoSuchCoin)
	}

	observer, ok := c.accountObservers[coinName]
	if !ok {
		return retErrAccountObserver{e: ErrCoinServiceNotImplemented}
	}
	return observer.WithContext(ctx).(IAccountObserver)
}
