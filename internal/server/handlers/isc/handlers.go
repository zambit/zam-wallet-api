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
	"strings"
)

const defaultCryptoCurrency = "BTC"

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
			params.Convert = "usd"
		}

		span.LogKV("user_phone", params.UserPhone)

		wtsCount, totalFiatBalance, totalDefaultCurrencyBalance := 0, new(decimal.Big), (*decimal.Big)(nil)
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
				addViewCurrencyCoin := true
				for _, name := range coinNames {
					if name == defaultCryptoCurrency {
						addViewCurrencyCoin = false
						break
					}
				}
				if addViewCurrencyCoin {
					coinNames = append(coinNames, defaultCryptoCurrency)
				}

				err = trace.InsideSpanE(
					ctx,
					"converting_coin_balances",
					func(ctx context.Context, span opentracing.Span) error {
						rates, err := cryptoConverter.GetMultiRate(ctx, coinNames, params.Convert)
						if err != nil {
							return err
						}

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

						totalDefaultCurrencyBalance = new(decimal.Big)
						totalDefaultCurrencyBalance = totalDefaultCurrencyBalance.Quo(
							totalFiatBalance, (*decimal.Big)(defaultCurrencyRate),
						)

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
				strings.ToLower(defaultCryptoCurrency): (*decimal2.View)(totalDefaultCurrencyBalance),
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
