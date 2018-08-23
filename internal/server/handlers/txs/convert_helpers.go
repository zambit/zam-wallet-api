package txs

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/common"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	ot "github.com/opentracing/opentracing-go"
	"strings"
)

// getRateForTx helper which queries tx coin rate for specified fiat currency
func getRateForTx(
	ctx context.Context,
	tx *processing.Tx,
	dstFiatCurrency string,
	converter convert.ICryptoCurrency,
) (bRate common.AdditionalRate, err error) {
	// perform convertation if this argument presented
	var additionalRate common.AdditionalRate
	if dstFiatCurrency == "" {
		dstFiatCurrency = common.DefaultFiatCurrency
	}
	dstFiatCurrency = strings.ToLower(dstFiatCurrency)

	err = trace.InsideSpanE(ctx, "converting_balance_to_fiat_currency", func(ctx context.Context, span ot.Span) error {
		span.LogKV("convert_to", dstFiatCurrency)
		span.LogKV("convert_from", tx.FromWallet.Coin.ShortName)

		// query rate with fallback currency
		rate, err := convert.GetRateDefaultFiat(
			converter, ctx, tx.FromWallet.Coin.ShortName, dstFiatCurrency, common.DefaultFiatCurrency,
		)
		if err != nil {
			return err
		}
		additionalRate = common.AdditionalRate{Rate: rate, FiatCurrency: dstFiatCurrency}
		return nil
	})
	return
}

// getRatesForTxs helper which queries txs coins rates for specified fiat currency
func getRatesForTxs(
	ctx context.Context,
	txs []processing.Tx,
	dstFiatCurrency string,
	converter convert.ICryptoCurrency,
) (bRates common.AdditionalRates, err error) {
	// perform convertation if this argument presented for all txs
	bRates = common.AdditionalRates{FiatCurrency: dstFiatCurrency}
	if len(txs) > 0 {
		if dstFiatCurrency == "" {
			dstFiatCurrency = common.DefaultFiatCurrency
		}

		dstFiatCurrency = strings.ToLower(dstFiatCurrency)

		err = trace.InsideSpanE(
			ctx, "converting_balances_to_fiat_currency",
			func(ctx context.Context, span ot.Span) error {
				// create a set of presented coins
				coinsSet := make(map[string]struct{})
				for _, tx := range txs {
					coinsSet[tx.FromWallet.Coin.ShortName] = struct{}{}
				}

				coinsList := make([]string, 0, len(coinsSet))
				for c := range coinsSet {
					coinsList = append(coinsList, c)
				}

				span.LogKV("convert_to", dstFiatCurrency)
				span.LogKV("convert_from", strings.Join(coinsList, ","))

				// query rates with fallback currency
				rates, err := convert.GetMultiRateDefaultFiat(
					converter, ctx, coinsList, dstFiatCurrency, common.DefaultFiatCurrency,
				)
				if err != nil {
					return err
				}
				bRates = common.AdditionalRates{MultiRate: rates, FiatCurrency: dstFiatCurrency}
				return nil
			},
		)
	}
	return
}
