package common

import (
	"git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	bdecimal "github.com/ericlagergren/decimal"
	"strings"
)

// MultiCurrencyBalance used to represent balance value in multiple currencies
type MultiCurrencyBalance map[string]*decimal.View

// AdditionalRate used to convert crypto-currency balance into additional currency
type AdditionalRate struct {
	*convert.Rate
	CoinCurrency string
	FiatCurrency string
}

var zeroDecimalView = (*decimal.View)(new(bdecimal.Big).SetFloat64(0))

// RepresentBalance represent given balance in specified currencies
func (ar *AdditionalRate) RepresentBalance(balance *bdecimal.Big) MultiCurrencyBalance {
	balances := map[string]*decimal.View{
		strings.ToLower(ar.CoinCurrency): (*decimal.View)(balance),
	}
	if ar.FiatCurrency != "" {
		var value *decimal.View
		if ar.Rate != nil {
			value = (*decimal.View)(ar.Convert(balance))
		} else {
			value = zeroDecimalView
		}
		balances[strings.ToLower(ar.FiatCurrency)] = value
	}
	return balances
}

// AdditionalRates same as AdditionalRate, but for multiple crypto-currency balances
type AdditionalRates struct {
	convert.MultiRate
	FiatCurrency string
}

// ForCoinCurrency return rate description for selected currency
func (ar *AdditionalRates) ForCoinCurrency(coinName string) AdditionalRate {
	return AdditionalRate{
		Rate:         ar.CurrencyRate(coinName),
		CoinCurrency: coinName,
		FiatCurrency: strings.ToLower(ar.FiatCurrency),
	}
}
