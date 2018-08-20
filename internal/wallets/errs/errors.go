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

	// ErrInvalidPhone returned when phone is invalid
	ErrInvalidPhone = errors.New("wallets: invalid user phone")

	// ErrSelfTxForbidden returned when self tx attempt detected
	ErrSelfTxForbidden = errors.New("wallets: self tx forbidden")

	// ErrNonPositiveAmount indicates invalid amount which less or equal to zero
	ErrNonPositiveAmount = errors.New("wallets: non positive")
)
