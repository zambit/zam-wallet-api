package processing

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type StepResources struct {
	Coordinator   nodes.ICoordinator
	BalanceHelper helpers.IBalance
}

// StepTx performs as much transaction steps as possible depends on current transaction state
func StepTx(ctx context.Context, dbTx *gorm.DB, tx *Tx, res *StepResources) (newTx *Tx, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "step_tx")
	defer span.Finish()

	var validateErrs error
	err = db.TransactionCtx(ctx, dbTx, func(ctx context.Context, dbTx *gorm.DB) error {
		span := opentracing.SpanFromContext(ctx)

		txForUpdate := &Tx{ID: tx.ID}
		err := dbTx.Set(
			"gorm:query_option", "FOR UPDATE",
		).Preload("FromWallet").Preload("ToWallet").Preload("Status").Preload("FromWallet.Coin").First(
			txForUpdate, txForUpdate,
		).Error
		if err != nil {
			return err
		}

		var nextStep = true
		// step inside loop until steps available
		for nextStep {
			// get state func
			f := getStateFunc(txForUpdate.Status.Name)
			// there is nothing more to do, returning
			if f == nil {
				break
			} else {
				var (
					newState         string
					stepValidateErrs error
				)
				newState, nextStep, stepValidateErrs, err = validateTxState(ctx, txForUpdate, res)
				if err != nil {
					return err
				}

				// query status explicitly, no clear way with gorm :(
				var stateModel TxStatus
				err = dbTx.Model(&stateModel).Where("name = ?", newState).First(&stateModel).Error
				if err != nil {
					return err
				}

				// update model
				txForUpdate.Status = &stateModel
				txForUpdate.StatusID = stateModel.ID
				err = dbTx.Model(txForUpdate).Update("StatusID", stateModel.ID).Error
				if err != nil {
					return err
				}

				if stepValidateErrs != nil {
					validateErrs = merrors.Append(validateErrs, stepValidateErrs)
				}
			}
		}

		if validateErrs != nil {
			span.LogKV("validation_errs", validateErrs)
			return nil
		}

		newTx = txForUpdate
		return nil
	})
	if err != nil {
		span.LogKV("err", err)
	} else if validateErrs != nil {
		err = validateErrs
	}
	return
}

//
type stateFunc func(ctx context.Context, tx *Tx, res *StepResources) (newState string, inWait bool, validateErrs, err error)

//
func getStateFunc(state string) stateFunc {
	switch state {
	case TxStateJustCreated:
		return validateTxState
	case TxStateProcessed, TxStateDeclined:
		return nil
	default:
		return nil
	}
}

func validateTxState(ctx context.Context, tx *Tx, res *StepResources) (newState string, nextStep bool, validateErrs, err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "validate_tx")
	defer span.Finish()

	// validate tx properties
	if tx.Amount == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx amount is missing"))
	}
	if tx.FromWallet == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx src wallet is missing"))
	}
	if validateErrs != nil {
		span.LogKV("msg", "internal validations failed, code is wrong!", "validation_errs", validateErrs)
		newState = TxStateJustCreated
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
		newState = TxStateProcessed
	}
	return
}
