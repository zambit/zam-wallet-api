package wallets

import (
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	ot "github.com/opentracing/opentracing-go"
	"net/http"
	"strings"
	"sync"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
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
		resp = ResponseFromWallet(wallet, AdditionalBalance{})

		return
	}
}

// GetFactory creates handler which used to query wallet which is specified by path param 'wallet_id', returns
// 'Response' on success.
func GetFactory(api *wallets.Api, converter helpers.ICoinConverter) base.HandlerFunc {
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
		var additionalBalance AdditionalBalance
		if params.Convert != "" {
			var err error

			span, ctx := ot.StartSpanFromContext(ctx, "converting_balance")
			span.LogKV("convert_to", params.Convert)
			trace.LogMsg(span, "converting wallet balance to additional currencies")

			var convertedBalance *decimal.Big
			convertedBalance, err = converter.ConvertToFiat(ctx, wallet.Coin.ShortName, wallet.Balance, params.Convert)
			if err != nil {
				trace.LogError(span, err)
			} else {
				additionalBalance = AdditionalBalance{
					Currency: strings.ToLower(params.Convert),
					Amount:   convertedBalance,
				}
			}
			span.Finish()
		}

		// prepare response body
		resp = ResponseFromWallet(wallet, additionalBalance)
		return
	}
}

// GetAllFactory
func GetAllFactory(api *wallets.Api, converter helpers.ICoinConverter) base.HandlerFunc {
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
		var additionalBalances map[int64]AdditionalBalance
		if params.Convert != "" && len(wts) > 0 {
			var err error

			additionalBalances = make(map[int64]AdditionalBalance, len(wts))

			span := span.Tracer().StartSpan("converting_balances_for_wallets", ot.ChildOf(span.Context()))
			span.LogKV("convert_to", params.Convert)
			trace.LogMsg(span, "converting wallet balance to additional currency")

			convert := func(w wallets.WalletWithBalance) AdditionalBalance {
				var convertedBalance *decimal.Big
				convertedBalance, err = converter.ConvertToFiat(ctx, w.Coin.ShortName, w.Balance, params.Convert)
				if err != nil {
					trace.LogError(span, err)
					return AdditionalBalance{}
				} else {
					return AdditionalBalance{
						Currency: strings.ToLower(params.Convert),
						Amount:   convertedBalance,
					}
				}
			}

			// since number of wallets highly limited, around 4-8 at all, it's more optimal to use goroutine for each
			// wallet request, not workers pool
			switch len(wts) {
			case 1:
				res := convert(wts[0])
				additionalBalances[wts[0].ID] = res
			default:
				type res struct {
					wID     int64
					balance AdditionalBalance
				}

				var wg sync.WaitGroup
				wg.Add(len(wts))
				resChan := make(chan res)

				for _, w := range wts {
					go func(w wallets.WalletWithBalance) {
						resChan <- res{
							wID:     w.ID,
							balance: convert(w),
						}
						wg.Done()
					}(w)
				}

				go func() {
					wg.Wait()
					close(resChan)
				}()

				for r := range resChan {
					additionalBalances[r.wID] = r.balance
				}
			}

			span.Finish()
		}

		// prepare response body
		resp = AllResponseFromWallets(wts, totalCount, hasNext, additionalBalances)
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
