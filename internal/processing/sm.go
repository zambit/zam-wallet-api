package processing

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
)

type SmResources struct {
	Coordinator   nodes.ICoordinator
	BalanceHelper helpers.IBalance
}

// StepTx performs as much transaction steps as possible depends on current transaction state
func StepTx(ctx context.Context, dbTx *gorm.DB, tx *Tx, res *SmResources) (newTx *Tx, validateErrs error, err error) {
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
type stateFunc func(ctx context.Context, tx *Tx, res *SmResources) (newState string, inWait bool, validateErrs, err error)

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

func recipientWalletCreated(ctx context.Context, tx *Tx, res *SmResources) (newState string, nextStep bool, validateErrs, err error) {
	// send transaction back to validation state due to sender balance can be changed while awaiting recipient wallet creation
	newState = TxStateValidate
	nextStep = true
	return
}

func validateTxState(ctx context.Context, tx *Tx, res *SmResources) (newState string, nextStep bool, validateErrs, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "validate_tx")
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
	totalBalance, err := res.BalanceHelper.TotalWalletBalanceCtx(ctx, tx.FromWallet)
	if err != nil {
		return
	}
	span.LogKV("wallet_total_balance", totalBalance)
	if totalBalance.Cmp(amount) < 0 {
		validateErrs = merrors.Append(validateErrs, ErrInsufficientFunds)
	}

	if validateErrs != nil {
		newState = TxStateDeclined
	} else {
		switch {
		case tx.ToWalletID != nil:
			newState = TxStateProcessed
		case tx.ToPhone != nil:
			newState = TxStateAwaitRecipient
		}
	}
	return
}
