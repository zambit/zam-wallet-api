package wallets

import (
	"errors"
	errors2 "git.zam.io/wallet-backend/common/pkg/errors"
	"git.zam.io/wallet-backend/wallet-api/services/wallets/providers"
	"io"
)

var (
	ErrNoSuchCoin = errors.New("no such coin found")

	ErrCoinUnavailable = errors.New("coin processing unavailable")

	ErrCoinIsUnsupported = errors.New("coin is unsupported")

	ErrCoinServiceNotImplemented = errors.New("coin service is not implemented")
)

// ICoordinator
type ICoordinator interface {
	// Dial coin for given params and add coin services to this coordinator
	//
	// If there is no actual implementation for required coin, ErrCoinIsUnsupported will be returned.
	Dial(coinName string, host, user, pass string, testnet bool) error

	// Close closes all connections
	Close() error

	// Generator returns generator which belongs to a specific coin or ErrNoSuchCoin
	Generator(coinName string) (IGenerator, error)
}

// New creates new default coordinator
func New() ICoordinator {
	return &coordinator{
		closers:    make(map[string]io.Closer),
		generators: make(map[string]IGenerator),
	}
}

// coordinator implements ICoordinator in straight way
type coordinator struct {
	closers    map[string]io.Closer
	generators map[string]IGenerator
}

// Dial lookup service provider registry, dial no safe with concurrent getters usage
func (c *coordinator) Dial(coinName string, host, user, pass string, testnet bool) error {
	provider, ok := providers.Get(coinName)
	if !ok {
		return ErrCoinIsUnsupported
	}

	services, err := provider.Dial(host, user, pass, testnet)
	if err != nil {
		return err
	}

	c.closers[coinName] = services

	if generator, ok := services.(IGenerator); ok {
		c.generators[coinName] = generator
	}

	return nil
}

// Close implements ICoordinator interface
func (c *coordinator) Close() error {
	var errs []error
	for _, closer := range c.closers {
		err := closer.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors2.MultiErrors(errs)
}

// Generator implements ICoordinator interface
func (c *coordinator) Generator(coinName string) (IGenerator, error) {
	if _, ok := c.closers[coinName]; !ok {
		return nil, ErrNoSuchCoin
	}

	generator, ok := c.generators[coinName]
	if !ok {
		return nil, ErrCoinServiceNotImplemented
	}

	return generator, nil
}
