package wallets

import (
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"net/http"
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
		userID, err := getUserID(c)
		if err != nil {
			return
		}

		// create wallet
		wallet, err := api.CreateWallet(userID, params.Coin, params.WalletName)

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
		resp = ResponseFromWallet(wallet)

		return
	}
}

// GetFactory creates handler which used to query wallet which is specified by path param 'wallet_id', returns
// 'Response' on success.
func GetFactory(api *wallets.Api) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		// parse wallet id path param
		walletID, walletIDValid := parseWalletIDView(c.Param("wallet_id"))
		if !walletIDValid {
			err = errWalletIDInvalid
			return
		}

		// extract user id
		userID, err := getUserID(c)
		if err != nil {
			return
		}

		// perform request
		wallet, err := api.GetWallet(userID, walletID)
		if err != nil {
			if err == errs.ErrNoSuchWallet {
				// invalid wallet id also set 404 error code
				err = errWalletIDNotFound
			}
			return
		}

		// prepare response body
		resp = ResponseFromWallet(wallet)
		return
	}
}

// GetAllFactory
func GetAllFactory(api *wallets.Api) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		params := DefaultGetAllRequest()
		// ignore error due to invalid query params just ignored
		c.BindQuery(&params)

		// parse cursor
		fromID, _ := parseWalletIDView(params.Cursor)

		// extract user id
		userID, err := getUserID(c)
		if err != nil {
			return
		}

		// query wallets
		wts, totalCount, hasNext, err := api.GetWallets(userID, params.ByCoin, fromID, params.Count)
		if err != nil {
			return
		}

		// prepare response body
		resp = AllResponseFromWallets(wts, totalCount, hasNext)
		return
	}
}

// utils
// getUserID extracts user id from context which must be attached by user middleware
func getUserID(c *gin.Context) (userID int64, err error) {
	userID, presented := middlewares.GetUserIDFromContext(c)
	if !presented {
		err = errUserMiddlewareMissing
	}
	return
}
