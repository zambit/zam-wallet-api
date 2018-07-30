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
	"strconv"
	"strings"
)

var (
	errUserMiddlewareMissing = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "",
	}
	errWalletIDInvalid = base.NewErrorsView("").AddField(
		"path", "wallet_id", "invalid wallet id",
	)
	errCoinInvalid = base.NewErrorsView("").AddField(
		"body", "coin", "invalid coin",
	)
)

// CreateFactory
func CreateFactory(d *db.Db, generator wallets.IGenerator) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		// bind params
		params := CreateRequest{}
		_, err = base.ShouldBindJSON(c, &params)
		if err != nil {
			return
		}

		// extract user id from context which must be attached by user middleware
		userID, presented := middlewares.GetUserIDFromContext(c)
		if !presented {
			err = errUserMiddlewareMissing
			return
		}

		// generate wallet address
		_, walletAddress, err := generator.Create(params.Coin, fmt.Sprintf("%d_%s", userID, params.Coin))

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

// GetFactory
func GetFactory(d *db.Db) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		// parse wallet id from passed as path param
		rawWalletID := c.Param("wallet_id")
		walletID, parseIntErr := strconv.ParseInt(rawWalletID, 10, 64)
		if parseIntErr != nil {
			err = errWalletIDInvalid
			return
		}

		// extract user id from context which must be attached by user middleware
		userID, presented := middlewares.GetUserIDFromContext(c)
		if !presented {
			err = errUserMiddlewareMissing
			return
		}

		var wallet models.Wallet
		err = d.Tx(func(tx db.ITx) error {
			wallet, err = models.GetWallet(tx, userID, walletID)
			return err
		})
		if err != nil {
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
		// TODO extract filter params

		// extract user id from context which must be attached by user middleware
		userID, presented := middlewares.GetUserIDFromContext(c)
		if !presented {
			err = errUserMiddlewareMissing
			return
		}

		var wallets []models.Wallet
		var totalCount int64
		err = d.Tx(func(tx db.ITx) error {
			wallets, totalCount, err = models.GetWallets(tx, userID)
			return err
		})
		if err != nil {
			return
		}

		// render response view
		resp = AllResponseFromWallets(wallets, totalCount)
		return
	}
}
