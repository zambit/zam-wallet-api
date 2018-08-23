package wallets

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/common"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/wallet-api/pkg/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	ot "github.com/opentracing/opentracing-go"
	"strings"
)

var (
	errWalletIDInvalid               = base.NewFieldErr("path", "wallet_id", "wallet id invalid")
	errWalletIDNotFound              = base.NewFieldErr("path", "wallet_id", "wallet not found")
	errWalletOfSuchCoinAlreadyExists = base.NewFieldErr("body", "coin", "wallet of such coin already exists")
	errCoinInvalid                   = base.NewFieldErr("body", "coin", "invalid coin name")
)

// CreateFactory creates handler which used to create wallet, accepting 'CreateRequest' like scheme and returns
// 'Response' on success.
func CreateFactory(api *wallets.Api) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		// bind params
		params := CreateRequest{}
		err = base.ShouldBindJSON(c, &params)
		if err != nil {
			if base.HaveFieldErr(err, "coin") {
				cErr := api.ValidateCoin(params.Coin)
				if cErr == errs.ErrNoSuchCoin {
					err = merrors.Append(err, errCoinInvalid)
				} else if cErr != nil {
					err = cErr
				}
			}
			return
		}

		// extract user id
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// create wallet
		wallet, err := api.CreateWallet(ctx, userPhone, params.Coin, params.WalletName)

		if err != nil {
			// coerce error
			switch err {
			case errs.ErrNoSuchCoin:
				err = errCoinInvalid
			case errs.ErrWalletCreationRejected:
				err = errWalletOfSuchCoinAlreadyExists
			}
			return
		}

		// prepare response body
		code = 201
		resp = ResponseFromWallet(wallet, common.AdditionalRate{})

		return
	}
}

// GetFactory creates handler which used to query wallet which is specified by path param 'wallet_id', returns
// 'Response' on success.
func GetFactory(api *wallets.Api, converter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		// parse wallet id path param
		walletID, walletIDValid := ParseWalletIDView(c.Param("wallet_id"))
		if !walletIDValid {
			err = errWalletIDInvalid
			return
		}
		span.LogKV("wallet_id", walletID)

		// extract user id
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// extract query params and ignore errors
		params := GetRequest{}
		c.BindQuery(&params)

		// perform request
		wallet, err := api.GetWallet(ctx, userPhone, walletID)
		if err != nil {
			if err == errs.ErrNoSuchWallet {
				// invalid wallet id also set 404 error code
				err = errWalletIDNotFound
			} else {
				trace.LogErrorWithMsg(span, err, "getting wallet error")
			}
			return
		}

		// coerce convert param
		if params.Convert == "" {
			params.Convert = common.DefaultFiatCurrency
		}
		additionalRate := common.AdditionalRate{FiatCurrency: params.Convert}

		trace.InsideSpanE(ctx, "converting_balance_to_fiat_currency", func(ctx context.Context, span ot.Span) error {
			span.LogKV("convert_to", params.Convert)
			span.LogKV("convert_from", wallet.Coin.ShortName)

			var err error
			additionalRate.Rate, err = convert.GetRateDefaultFiat(
				converter, ctx, wallet.Coin.ShortName, params.Convert, common.DefaultFiatCurrency,
			)
			return err
		})

		// prepare response body
		resp = ResponseFromWallet(wallet, additionalRate)
		return
	}
}

// GetAllFactory
func GetAllFactory(api *wallets.Api, converter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		params := GetAllRequest{Count: 10}
		// ignore error due to invalid query params just ignored
		c.BindQuery(&params)

		// parse cursor
		fromID, _ := ParseWalletIDView(params.Cursor)
		span.LogKV("from_id", fromID)

		// extract user id
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// query wallets
		wts, totalCount, hasNext, err := api.GetWallets(ctx, userPhone, params.ByCoin, fromID, params.Count)
		if err != nil {
			trace.LogErrorWithMsg(span, err, "error getting wallets")
			return
		}

		// coerce convert param
		if params.Convert == "" {
			params.Convert = common.DefaultFiatCurrency
		}
		// perform convertation if this argument presented for all wallets
		additionalRates := common.AdditionalRates{FiatCurrency: params.Convert}
		if len(wts) > 0 {
			trace.InsideSpanE(ctx, "converting_balances_to_fiat_currency", func(ctx context.Context, span ot.Span) error {
				nonZeroWts := filterNonZeroWallets(wts)
				if len(nonZeroWts) == 0 {
					return nil
				}
				coinsList := make([]string, len(nonZeroWts))
				for i, w := range nonZeroWts {
					coinsList[i] = w.Coin.ShortName
				}

				span.LogKV("convert_to", params.Convert)
				span.LogKV("convert_from", strings.Join(coinsList, ","))

				var err error
				additionalRates.MultiRate, err = convert.GetMultiRateDefaultFiat(
					converter, ctx, coinsList, params.Convert, common.DefaultFiatCurrency,
				)
				return err
			})
		}

		// prepare response body
		resp = AllResponseFromWallets(wts, totalCount, hasNext, additionalRates)
		return
	}
}

// utils
func filterNonZeroWallets(wts []wallets.WalletWithBalance) []wallets.WalletWithBalance {
	nWts := make([]wallets.WalletWithBalance, 0, len(wts))
	for _, w := range wts {
		if w.Balance != nil && w.Balance.Sign() != 0 {
			nWts = append(nWts, w)
		}
	}
	return nWts
}
