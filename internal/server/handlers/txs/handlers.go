package txs

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"

	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/wallet-api/pkg/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"net/http"
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
	errWrongTxAmount  = base.NewFieldErr("body", "amount", "must be greater then zero")
	errTxAmountToBig  = base.NewFieldErr("body", "amount", "such a great value can not be accepted")
	errNoSuchWallet   = base.NewFieldErr("body", "wallet_id", "no such wallet")
	errRecipientIsYou = base.NewFieldErr("body", "recipient", "you can't send amount to your self")
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
			err = coerceProcessingErrs(err)
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

func coerceProcessingErrs(err error) error {
	if errors, ok := err.(merrors.Errors); ok {
		for i, e := range errors {
			errors[i] = coerceErr(e)
		}
		return errors
	}
	return coerceErr(err)
}

func coerceErr(e error) (newE error) {
	switch e {
	case errs.ErrNoSuchWallet:
		newE = errNoSuchWallet
	case processing.ErrInsufficientFunds:
		newE = errInsufficientFunds
	case errs.ErrNonPositiveAmount:
		newE = errWrongTxAmount
	case errs.ErrSelfTxForbidden:
		newE = errRecipientIsYou
	case processing.ErrTxAmountToBig:
		newE = errTxAmountToBig
	default:
		newE = e
	}
	return
}
