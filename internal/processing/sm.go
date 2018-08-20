package processing

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type smResources struct {
	BalanceHelper      helpers.IBalance
	TxEventNotificator isc.ITxsEventNotificator
}

// StepTx performs as much transaction steps as possible depends on current transaction state
func StepTx(ctx context.Context, dbTx *gorm.DB, tx *Tx, res *smResources) (newTx *Tx, validateErrs error, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "step_tx")
	defer span.Finish()

	span.LogKV("tx_id", tx.ID)

	var nextStep = true
	// step inside loop until steps available
	for stepNum := 0; nextStep; stepNum++ {
		trace.InsideSpan(ctx, "step_evaluation", func(ctx context.Context, span opentracing.Span) {
			stateName := tx.Status.Name

			// get state func
			f, fName := getStateFunc(stateName), getStateFuncName(stateName)

			span.LogKV("step_func", fName, "step_num", stepNum, "state_name", stateName)

			// there is nothing more to do, returning
			if f == nil {
				nextStep = false
				return
			}

			var (
				newState         string
				stepValidateErrs error
			)
			newState, nextStep, stepValidateErrs, err = f(ctx, tx, res)
			if err != nil {
				return
			}

			span.LogKV("new_state", newState, "is_stepping_further", nextStep)

			if stepValidateErrs != nil {
				trace.LogErrorWithMsg(span, stepValidateErrs, "step validation errors")
			}

			// query status explicitly, no clear way with gorm :(
			var stateModel TxStatus
			err = dbTx.Model(&stateModel).Where("name = ?", newState).First(&stateModel).Error
			if err != nil {
				return
			}

			// update model
			tx.Status = &stateModel
			tx.StatusID = stateModel.ID
			err = dbTx.Model(tx).Update("StatusID", stateModel.ID).Error
			if err != nil {
				trace.LogErrorWithMsg(span, err, "tx updating error")
				return
			}

			if stepValidateErrs != nil {
				validateErrs = merrors.Append(validateErrs, stepValidateErrs)
			}
		})
	}
	if err != nil {
		return
	}
	newTx = tx
	return
}

//
type stateFunc func(ctx context.Context, tx *Tx, res *smResources) (
	newState string,
	inWait bool,
	validateErrs error,
	err error,
)

//
func getStateFunc(state string) stateFunc {
	switch state {
	case TxStateValidate:
		return validateTxState
	case TxStateAwaitRecipient:
		return recipientWalletCreated
	case TxStateProcessed, TxStateDeclined:
		return nil
	default:
		return nil
	}
}

func getStateFuncName(state string) string {
	switch state {
	case TxStateValidate:
		return "validating_tx"
	case TxStateAwaitRecipient:
		return "await_recipient"
	case TxStateProcessed, TxStateDeclined:
		return "noop"
	default:
		return "noop"
	}
}

func recipientWalletCreated(ctx context.Context, tx *Tx, res *smResources) (newState string, nextStep bool, validateErrs, err error) {
	// we don't need to verify sender balance again because tx in TxStateAwaitRecipient reserves it's amount
	newState = TxStateProcessed
	nextStep = true
	return
}

func validateTxState(ctx context.Context, tx *Tx, res *smResources) (newState string, nextStep bool, validateErrs, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "validate_tx")
	defer span.Finish()

	// validate tx properties
	if tx.Amount == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx amount is missing"))
	}
	if tx.FromWallet == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx src wallet is missing"))
	}
	if tx.ToPhone == nil && tx.ToWalletID == nil {
		validateErrs = merrors.Append(
			validateErrs,
			errors.New("either to_phone and to_wallet is empty, at least one should ne provided"),
		)
	}
	if validateErrs != nil {
		newState = TxStateDeclined
		return
	}

	coinName := tx.FromWallet.Coin.ShortName
	amount := tx.Amount.V

	// forbid self transactions
	switch {
	case tx.ToWalletID != nil && tx.FromWalletID == *tx.ToWalletID:
		validateErrs = merrors.Append(validateErrs, ErrSelfTxForbidden)
	case tx.ToPhone != nil && tx.FromWallet.UserPhone == *tx.ToPhone:
		validateErrs = merrors.Append(validateErrs, ErrSelfTxForbidden)
	}

	// query wallet balance, tx amount should not exceed the value we can ensure, return amount to big err in such case
	generalBalance, err := res.BalanceHelper.AccountBalanceCtx(ctx, coinName)
	if err != nil {
		return
	}
	span.LogKV("account_balance", generalBalance)
	if generalBalance.Cmp(amount) < 0 {
		validateErrs = merrors.Append(validateErrs, ErrTxAmountToBig)
	}

	// tx amount should no exceed total wallet balance, return insufficient funds in such case
	walletTotalBalance, err := res.BalanceHelper.TotalWalletBalanceCtx(ctx, tx.FromWallet)
	if err != nil {
		return
	}
	span.LogKV("wallet_total_balance", walletTotalBalance)
	if walletTotalBalance.Cmp(amount) < 0 {
		validateErrs = merrors.Append(validateErrs, ErrInsufficientFunds)
	}

	// wallet total balance should not exceed general balance
	if walletTotalBalance.Cmp(generalBalance) > 0 {
		validateErrs = merrors.Append(validateErrs, ErrInvalidWalletBalance)
	}

	if validateErrs != nil {
		newState = TxStateDeclined
	} else {
		switch {
		case tx.ToWalletID != nil:
			newState = TxStateProcessed
		case tx.ToPhone != nil:
			trace.InsideSpan(ctx, "sending_await_recipient_notification", func(ctx context.Context, span opentracing.Span) {
				notifErr := res.TxEventNotificator.AwaitRecipient(isc.TxEventPayload{
					Coin:           tx.FromWallet.Coin.ShortName,
					FromWalletName: tx.FromWallet.Name,
					FromPhone:      tx.FromWallet.UserPhone,
					Amount:         tx.Amount.V,
					ToPhone:        *tx.ToPhone,
				})
				if notifErr != nil {
					trace.LogError(span, notifErr)
					err = notifErr
				}
			})
			newState = TxStateAwaitRecipient
		}
	}
	return
}
