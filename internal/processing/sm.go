package processing

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type smResources struct {
	Coordinator        nodes.ICoordinator
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
			stateName := tx.StateName()

			// get state func
			f, fName := getStateFunc(stateName)

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
			newState, nextStep, stepValidateErrs, err = f(ctx, dbTx, tx, res)
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
type stateFunc func(ctx context.Context, dbTx *gorm.DB, tx *Tx, res *smResources) (
	newState string,
	inWait bool,
	validateErrs error,
	err error,
)

//
func getStateFunc(state string) (stateFunc, string) {
	switch state {
	case TxStateValidate:
		return validateTxState, "validating_tx"
	case TxStateAwaitRecipient:
		return recipientWalletCreated, "await_recipient"
	case TxStateProcessed, TxStateDeclined:
		return nil, "noop"
	default:
		return nil, "noop"
	}
}

func recipientWalletCreated(
	ctx context.Context,
	dbTx *gorm.DB,
	tx *Tx,
	res *smResources,
) (newState string, nextStep bool, validateErrs, err error) {
	// we don't need to verify sender balance again because tx in TxStateAwaitRecipient reserves it's amount
	newState = TxStateProcessed
	nextStep = true
	return
}

func validateTxState(
	ctx context.Context,
	dbTx *gorm.DB,
	tx *Tx,
	res *smResources,
) (newState string, nextStep bool, validateErrs, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "validate_tx")
	defer span.Finish()

	// validate tx properties
	if tx.Amount == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx amount is missing"))
	}
	if tx.FromWallet == nil {
		validateErrs = merrors.Append(validateErrs, errors.New("tx src wallet is missing"))
	}
	if tx.ToPhone == nil && tx.ToWalletID == nil && tx.ToAddress == nil {
		validateErrs = merrors.Append(
			validateErrs,
			errors.New("all to_phone, to_wallet and to_address is empty, at least one should ne provided"),
		)
	}
	if validateErrs != nil {
		newState = TxStateDeclined
		return
	}

	coinName := tx.CoinName()
	amount := tx.Amount.V

	// forbid self transactions
	if tx.IsSelfTx() {
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
		case tx.SendByWallet():
			newState = TxStateProcessed
		case tx.SendByPhone():
			// TODO such async actions should be performed in individual state handler with individual state name
			// because of savepoints which will be used in future to protect each state
			trace.InsideSpan(ctx, "sending_await_recipient_notification", func(ctx context.Context, span opentracing.Span) {
				notifErr := res.TxEventNotificator.AwaitRecipient(isc.TxEventPayload{
					Coin:           tx.CoinName(),
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
		case tx.SendByAddress():
			var txHash string
			err = trace.InsideSpanE(ctx, "sending_tx", func(ctx context.Context, span opentracing.Span) error {
				var err error
				txHash, err = res.Coordinator.TxsSender(tx.CoinName()).Send(
					ctx, tx.FromWallet.Address, *tx.ToAddress, tx.Amount.V,
				)
				return err
			})
			if err != nil {
				if err == nodes.ErrAddressInvalid {
					// return as validation err rather the ordinal error to save this transaction in txs history
					err = nil
					validateErrs = ErrInvalidAddress
					newState = TxStateDeclined
				}
				return
			}

			// create external tx
			err = dbTx.Create(&TxExternal{
				Tx:        tx,
				Hash:      txHash,
				Recipient: *tx.ToAddress,
			}).Error
			if err != nil {
				return
			}

			newState = TxStateAwaitConfirmations
		}
	}
	return
}
