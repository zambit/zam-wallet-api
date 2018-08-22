package convert

import (
	"context"
	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
	"strings"
)

var (
	// ErrCryptoCurrencyName when coin name is invalid
	ErrCryptoCurrencyName = errors.New("converter: crypto-currency name is invalid")

	// ErrFiatCurrencyName when currency name is invalid
	ErrFiatCurrencyName = errors.New("converter: fiat-currency name is invalid")

	// ErrUnavailable service not available, usually will be wrapped inside another error describing
	ErrUnavailable = errors.New("convert: unavailable")
)

// Rate used to perform conversion
type Rate decimal.Big

// Convert calculate result amount
func (s *Rate) Convert(amount *decimal.Big) *decimal.Big {
	return new(decimal.Big).Mul(amount, (*decimal.Big)(s))
}

// ReverseConvert performs reverse conversion
func (s *Rate) ReverseConvert(amount *decimal.Big) *decimal.Big {
	return new(decimal.Big).Quo(amount, (*decimal.Big)(s))
}

// MultiRate holds rates for multiple currencies
type MultiRate map[string]Rate

// CurrencyRate returns rate for currency, nil if no such exists
func (mr MultiRate) CurrencyRate(currency string) *Rate {
	currency = strings.ToUpper(currency)
	rate, ok := mr[currency]
	if !ok {
		return nil
	}
	return &rate
}

// ICryptoCurrency uses external real-time service to convert some coin amount into equivalent of fiat value
//
// All methods, which accept context, also supports deadlines and cancel channels
type ICryptoCurrency interface {
	// GetRate returns rate which should be used to convert from coin specified by short name to fiat currency
	// specified by short name. If coin name is invalid, returns ErrCryptoCurrencyName, if currency name is invalid,
	// returns ErrFiatCurrencyName. Both currency and coin names are case insensitive.
	GetRate(ctx context.Context, coinName string, dstCurrencyName string) (rate *Rate, err error)

	// GetMultiRate same as GetRate, but generate rates for multiple source coins
	GetMultiRate(ctx context.Context, coinNames[]string, dstCurrencyName string) (mr MultiRate, err error)
}
