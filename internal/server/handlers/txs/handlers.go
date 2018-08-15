package txs

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"

	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"net/http"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	)

var (
	errUserMiddlewareMissing = base.ErrorView{
		Code:    http.StatusInternalServerError,
		Message: "user middleware is missing",
	}
	errInsufficientFunds = base.ErrorView{
		Code:    http.StatusBadRequest,
		Message: "insufficient funds",
	}
	errNoSuchWallet = base.NewFieldErr("body", "wallet_id", "no such wallet")
)

// SendFactory
func SendFactory(walletApi *wallets.Api) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		params := SendRequest{}
		err = base.ShouldBindJSON(c, &params)
		if err != nil {
			return
		}

		span.LogKV(
			"wallet_id", params.WalletID,
			"recipient", params.Recipient,
			"amount", params.Amount,
		)

		// extract user phone
		userPhone, err := getUserPhone(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// try send money
		tx, err := walletApi.SendToPhone(ctx, userPhone, params.WalletID, params.Recipient, (*decimal.Big)(params.Amount))
		if err != nil {
			if err == errs.ErrNoSuchWallet {
				err = errNoSuchWallet
			} else if err == processing.ErrInsufficientFunds {
				err = errInsufficientFunds
			}
			return
		}

		// render response converting db format into api format
		resp = SendResponse{Transaction: ToView(tx, userPhone)}
		return
	}
}

func getUserPhone(c *gin.Context) (userPhone string, err error) {
	userPhone, presented := middlewares.GetUserPhoneFromContext(c)
	if !presented {
		err = errUserMiddlewareMissing
	}
	return
}
