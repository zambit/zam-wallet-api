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
	errWalletOfSuchCoinAlreadyExists = base.NewErrorsView("").AddField(
		"body", "coin", "wallet of such coin already exists",
	)
	errCoinInvalidDescr = base.FieldErrorDescr{
		Name: "name", Input: "body", Message: "invalid name",
	}
	errCoinInvalid = base.NewErrorsView("").AddFieldDescr(errCoinInvalidDescr)
)

func init() {
	// do it with init func due to bad base errors design, anyway it will be reworked soon
	errWalletIDNotFound.Code = http.StatusNotFound
}

// CreateFactory creates handler which used to create wallet, accepting 'CreateRequest' like scheme and returns
// 'Response' on success.
func CreateFactory(d *db.Db, coordinator wallets.ICoordinator) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		// bind params
		params := CreateRequest{}
		fErr, err := base.ShouldBindJSON(c, &params)
		if err != nil {
			lookupCoinErr := true
			for _, f := range fErr.Fields {
				if f.Name == "name" {
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

		// validate name name
		_, err = models.GetCoin(d, params.Coin)
		if err != nil {
			if err == models.ErrNoSuchCoin {
				err = errCoinInvalid
			}
			return
		}

		// validate name and get generator for specific name using coordinator
		generator, err := coordinator.Generator(params.Coin)
		if err != nil {
			if err == wallets.ErrNoSuchCoin {
				err = errCoinInvalid
			}
			return
		}

		var wallet models.Wallet
		err = d.Tx(func(tx db.ITx) (err error) {
			// since we wouldn't allow an user to create multiple wallets of
			// same name here we relies onto unique user/name constraint
			// so concurrent attempt to create next wallets with duplicated pairs
			// will be locked until first occurred transaction will be committed (in such case
			// constraint violation will occurs) or rollbacked (in such case wallet will be successfully
			// inserted)
			//
			// while other transactions hungs on this call we may safely generate wallet address (we sure
			// that no concurrent call on same user/name pair will occurs between insert and update, also
			// commit will be successful)
			//
			// TODO commit may be failed due to connection issues (for example), so wallet address will be generated, but no appropriate record occurs
			wallet, err = models.CreateWallet(
				tx, models.Wallet{
					UserID: userID,
					Coin: models.Coin{
						ShortName: params.Coin,
					},
					Name: fmt.Sprintf("%s wallet", strings.ToUpper(params.Coin)),
				},
			)
			if err != nil {
				switch err {
				case models.ErrNoSuchCoin:
					err = errCoinInvalid
				case models.ErrWalletCreationRejected:
					err = errWalletOfSuchCoinAlreadyExists
				}
				return
			}

			// after wallet was successfully created we may generate new wallet address
			wallet.Address, err = generator.Create()
			if err != nil {
				return
			}

			// then update wallet to new address
			err = models.UpdateWallet(tx, wallet.ID, &models.WalletDiff{Address: &wallet.Address})

			return
		})
		if err != nil {
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
		var hasNext bool
		err = d.Tx(func(tx db.ITx) error {
			wts, totalCount, hasNext, err = models.GetWallets(tx, userID, walletsFilters)
			return err
		})
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
