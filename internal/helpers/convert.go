package helpers

import (
	"context"
	"github.com/ericlagergren/decimal"
	"github.com/pkg/errors"
)

var (
	// ErrCryptoCurrencyName when coin name is invalid
	ErrCryptoCurrencyName = errors.New("converter: crypto-currency name is invalid")

	// ErrFiatCurrencyName when currency name is invalid
	ErrFiatCurrencyName = errors.New("converter: fiat-currency name is invalid")

	// ErrUnavailable service not available, usually will be wrapped inside another error describing
	ErrUnavailable = errors.New("convert: unavailable")
)

// ICoinConverter uses external real-time service to convert some coin amount into equivalent of fiat value
type ICoinConverter interface {
	// ConvertToFiat converts amount of coin name (specified by three letter short name) into dst fiat currency
	// specified also by 3 letter short name. Returns result and error. If coin name is invalid, returns ErrCryptoCurrencyName,
	// if currency name is invalid, returns ErrFiatCurrencyName. Both currency and coin names are case insensitive.
	ConvertToFiat(
		ctx context.Context,
		coinName string,
		amount *decimal.Big,
		dstCurrencyName string,
	) (fiatAmount *decimal.Big, err error)
}
