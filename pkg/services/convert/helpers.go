package convert

import "context"

// GetRateDefaultFiat helper which retries GetRate call in case of wrong fiat currency name with fallback argument
func GetRateDefaultFiat(
	converter ICryptoCurrency,
	ctx context.Context, coinName string,
	dstCurrencyName, fallbackFiatCurrency string,
) (rate *Rate, err error) {
	rate, err = converter.GetRate(ctx, coinName, dstCurrencyName)
	if err == ErrFiatCurrencyName {
		rate, err = converter.GetRate(ctx, coinName, fallbackFiatCurrency)
	}
	return
}

// GetMultiRateDefaultFiat helper which retries GetMultiRate call in case of wrong fiat currency name with fallback
// argument
func GetMultiRateDefaultFiat(
	converter ICryptoCurrency,
	ctx context.Context, coinNames []string,
	dstCurrencyName, fallbackFiatCurrency string,
) (mr MultiRate, err error) {
	mr, err = converter.GetMultiRate(ctx, coinNames, dstCurrencyName)
	if err == ErrFiatCurrencyName {
		mr, err = converter.GetMultiRate(ctx, coinNames, fallbackFiatCurrency)
	}
	return
}
