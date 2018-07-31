package wallets

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/models"
	"git.zam.io/wallet-backend/wallet-api/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/services/wallets"
	"git.zam.io/wallet-backend/web-api/db"
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

var (
	errUserMiddlewareMissing = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "user middleware is missing",
	}
	errWalletIDInvalid = base.NewErrorsView("").AddField(
		"path", "wallet_id", "wallet id invalid",
	)
	errWalletIDNotFound = base.NewErrorsView("").AddField(
		"path", "wallet_id", "wallet not found",
	)
	errCoinInvalidDescr = base.FieldErrorDescr{
		Name: "coin", Input: "body", Message: "invalid coin",
	}
	errCoinInvalid = base.NewErrorsView("").AddFieldDescr(errCoinInvalidDescr)
)

func init() {
	// do it with init func due to bad base errors design, anyway it will be reworked soon
	errWalletIDNotFound.Code = http.StatusNotFound
}

// CreateFactory creates handler which used to create wallet, accepting 'CreateRequest' like scheme and returns
// 'Response' on success.
func CreateFactory(d *db.Db, generator wallets.IGenerator) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		// bind params
		params := CreateRequest{}
		fErr, err := base.ShouldBindJSON(c, &params)
		if err != nil {
			lookupCoinErr := true
			for _, f := range fErr.Fields {
				if f.Name == "coin" {
					lookupCoinErr = false
					break
				}
			}
			if lookupCoinErr {
				_, err = models.GetCoin(d, params.Coin)
				if err == models.ErrNoSuchCoin {
					fErr.AddFieldDescr(errCoinInvalidDescr)
				}
				err = fErr
			}
			return
		}

		// extract user id
		userID, err := getUserID(c)
		if err != nil {
			return
		}

		// validate coin name
		_, err = models.GetCoin(d, params.Coin)
		if err != nil {
			if err == models.ErrNoSuchCoin {
				err = errCoinInvalid
			}
			return
		}

		// generate wallet address
		_, walletAddress, err := generator.Create(params.Coin, fmt.Sprintf("%d_%s", userID, params.Coin))
		if err != nil {
			return
		}

		// generate wallet struct
		wallet := models.Wallet{
			UserID: userID,
			Coin: models.Coin{
				ShortName: params.Coin,
			},
			Name:    fmt.Sprintf("%s wallet", strings.ToUpper(params.Coin)),
			Address: walletAddress,
		}

		err = d.Tx(func(tx db.ITx) error {
			wallet, err = models.CreateWallet(tx, wallet)
			return err
		})
		if err != nil {
			if err == models.ErrNoSuchCoin {
				err = errCoinInvalid
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
func GetFactory(d *db.Db) base.HandlerFunc {
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

		var wallet models.Wallet
		err = d.Tx(func(tx db.ITx) error {
			wallet, err = models.GetWallet(tx, userID, walletID)
			return err
		})
		if err != nil {
			if err == models.ErrNoSuchWallet {
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
func GetAllFactory(d *db.Db) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		params := DefaultGetAllRequest()
		// ignore error due to invalid query params just ignored
		c.BindQuery(&params)

		// map then to filters description
		walletsFilters := models.GetWalletFilters{
			ByCoin: params.ByCoin,
			Count:  params.Count,
		}
		// parse cursor
		fromID, valid := parseWalletIDView(params.Cursor)
		if !valid {
			walletsFilters.FromID = fromID
		}

		// extract user id
		userID, err := getUserID(c)
		if err != nil {
			return
		}

		var wts []models.Wallet
		var totalCount int64
		err = d.Tx(func(tx db.ITx) error {
			wts, totalCount, err = models.GetWallets(tx, userID, walletsFilters)
			return err
		})
		if err != nil {
			return
		}

		// prepare response body
		resp = AllResponseFromWallets(wts, totalCount)
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
