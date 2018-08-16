package wallets

import (
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/pkg/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	ot "github.com/opentracing/opentracing-go"
	"net/http"
	"strings"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"context"
)

var (
	errUserMiddlewareMissing = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "user middleware is missing",
	}
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
		userPhone, err := getUserPhone(c)
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
		resp = ResponseFromWallet(wallet, AdditionalRate{})

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
		walletID, walletIDValid := parseWalletIDView(c.Param("wallet_id"))
		if !walletIDValid {
			err = errWalletIDInvalid
			return
		}
		span.LogKV("wallet_id", walletID)

		// extract user id
		userPhone, err := getUserPhone(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// extract query params and ignore errors
		params := DefaultGetRequest()
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

		// perform convertation if this argument presented
		var additionalRate AdditionalRate
		if params.Convert != "" {
			trace.InsideSpanE(ctx, "converting_balance_to_fiat_currency", func(ctx context.Context, span ot.Span) error {
				span.LogKV("convert_to", params.Convert)
				span.LogKV("convert_from", wallet.Coin.ShortName)

				rate, err := converter.GetRate(ctx, wallet.Coin.ShortName, params.Convert)
				if err != nil {
					return err
				}
				additionalRate = AdditionalRate{Rate: rate, Currency: params.Convert}
				return nil
			})
		}

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

		params := DefaultGetAllRequest()
		// ignore error due to invalid query params just ignored
		c.BindQuery(&params)
		span.LogKV("params", params)

		// parse cursor
		fromID, _ := parseWalletIDView(params.Cursor)
		span.LogKV("from_id", fromID)

		// extract user id
		userPhone, err := getUserPhone(c)
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

		// perform convertation if this argument presented for all wallets
		var additionalRates AdditionalRates
		if params.Convert != "" && len(wts) > 0 {
			nonZeroWts := filterNonZeroWallets(wts)
			trace.InsideSpanE(ctx, "converting_balances_to_fiat_currency", func(ctx context.Context, span ot.Span) error {
				coinsList := make([]string, len(nonZeroWts))
				for i, w := range nonZeroWts {
					coinsList[i] = w.Coin.ShortName
				}

				span.LogKV("convert_to", params.Convert)
				span.LogKV("convert_from", strings.Join(coinsList, ", "))

				rates, err := converter.GetMultiRate(ctx, coinsList, params.Convert)
				if err != nil {
					return err
				}
				additionalRates = AdditionalRates{MultiRate: rates, Currency: params.Convert}
				return nil
			})
		}

		// prepare response body
		resp = AllResponseFromWallets(wts, totalCount, hasNext, additionalRates)
		return
	}
}

// utils
// getUserPhone extracts user id from context which must be attached by user middleware
func getUserPhone(c *gin.Context) (userPhone string, err error) {
	userPhone, presented := middlewares.GetUserPhoneFromContext(c)
	if !presented {
		err = errUserMiddlewareMissing
	}
	return
}

func filterNonZeroWallets(wts []wallets.WalletWithBalance) []wallets.WalletWithBalance {
	nWts := make([]wallets.WalletWithBalance, 0, len(wts))
	for _, w := range wts {
		if w.Balance != nil && w.Balance.Sign() != 0 {
			nWts = append(nWts, w)
		}
	}
	return nWts
}