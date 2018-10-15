package isc

import (
	"context"
	decimal2 "git.zam.io/wallet-backend/common/pkg/types/decimal"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"strings"
)

const (
	defaultCryptoCurrency = "BTC"
	defaultFeatCurrency   = "usd"
)

// UserStatFactory returns total user wallets balances
func UserStatFactory(api *wallets.Api, cryptoConverter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		// bind query params
		params := UserStatRequest{}
		err = c.BindQuery(&params)
		if err != nil {
			return
		}
		if params.Convert == "" {
			params.Convert = defaultCryptoCurrency
		}

		span.LogKV("user_phone", params.UserPhone)

		wtsCount, totalFiatBalance, totalBalanceInDefCurr := 0, new(decimal.Big), new(decimal.Big)
		err = trace.InsideSpanE(
			ctx,
			"querying_user_wallets_balance",
			func(ctx context.Context, span opentracing.Span) error {
				wts, _, _, err := api.GetWallets(ctx, params.UserPhone, "", 0, 0)
				if err != nil {
					return err
				}

				span.LogKV("wts_num", len(wts))

				if len(wts) == 0 {
					return nil
				}
				wtsCount = len(wts)

				// calculate total fiat balance
				coinNames := nonZeroWalletsCoins(wts)
				if len(coinNames) == 0 {
					return nil
				}
				logrus.Info(coinNames)

				addDefaultCurrencyCoin := true
				for _, name := range coinNames {
					if name == defaultCryptoCurrency {
						addDefaultCurrencyCoin = false
						break
					}
				}
				if addDefaultCurrencyCoin {
					coinNames = append(coinNames, defaultCryptoCurrency)
				}

				logrus.Info(coinNames)

				err = trace.InsideSpanE(
					ctx,
					"converting_coin_balances",
					func(ctx context.Context, span opentracing.Span) error {
						rates, err := convert.GetMultiRateDefaultFiat(
							cryptoConverter, ctx, coinNames, params.Convert, defaultFeatCurrency,
						)
						if err != nil {
							return err
						}
						logrus.Info(rates)

						// calculate total fiat balance
						for _, w := range wts {
							if w.Balance == nil || w.Balance.Sign() == 0 {
								continue
							}

							totalFiatBalance = totalFiatBalance.Add(
								totalFiatBalance, rates.CurrencyRate(w.Coin.ShortName).Convert(w.Balance),
							)
						}

						// calculate total btc balance by reverse converting total fiat balance
						defaultCurrencyRate := rates.CurrencyRate(defaultCryptoCurrency)
						totalBalanceInDefCurr = defaultCurrencyRate.ReverseConvert(totalFiatBalance)

						return nil
					},
				)
				if err != nil {
					return err
				}

				return nil
			},
		)
		if err != nil {
			return
		}

		// prepare response
		resp = UserStatsResponseView{
			Count: wtsCount,
			TotalBalance: map[string]*decimal2.View{
				strings.ToLower(defaultCryptoCurrency): (*decimal2.View)(totalBalanceInDefCurr),
				strings.ToLower(params.Convert):        (*decimal2.View)(totalFiatBalance),
			},
		}

		return
	}
}

// utils
func nonZeroWalletsCoins(wts []wallets.WalletWithBalance) []string {
	nWts := make([]string, 0, len(wts))
	for _, w := range wts {
		if w.Balance != nil && w.Balance.Sign() != 0 {
			nWts = append(nWts, w.Coin.ShortName)
		}
	}
	return nWts
}
