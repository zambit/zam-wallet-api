package errs

import (
	"errors"
)

var (
	// ErrNoSuchCoin returned when invalid coin name is specified
	ErrNoSuchCoin = errors.New("such coin name is unexpected")

	// ErrNoSuchWallet returned when no wallet found for specified criteria
	ErrNoSuchWallet = errors.New("no such wallet found")

	// ErrWalletCreationRejected returned when wallet connot be created due to specific values limitations
	ErrWalletCreationRejected = errors.New("wallet creation rejected due to params")

	// ErrNotInsufficientFunds anyone knows what this error means.
	ErrNotInsufficientFunds = errors.New("processing: insufficient funds")
)
