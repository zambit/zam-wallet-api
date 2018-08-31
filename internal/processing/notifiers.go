package processing

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"strings"
	"sync"
)

// IConfirmationNotifier used to notify processing about confirmation events in block-chain
type IConfirmationNotifier interface {
	// OnNewConfirmation notifies that external transactions confirmation can be checked
	//
	// TODO this method should be deleted in future, because, ideally, nodes gate should allow to subscribe on
	// transaction confirmation change event and emit them when transaction may be treated as successful.
	// Here i suppose that ideally it should be accomplished using callback with this signature
	// 'OnTxConfirmed(tx *TxExternal) error' which will be called when external tx reach necessary count
	// of confirmations.
	OnNewConfirmation(ctx context.Context, coinName string) error
}

// ConfirmationNotifier is IConfirmationNotifier implementation
type ConfirmationNotifier struct {
	database    *gorm.DB
	coordinator nodes.ICoordinator
}

// NewConfirmationsNotifier creates new confirmations notifier
func NewConfirmationsNotifier(db *gorm.DB, coordinator nodes.ICoordinator) IConfirmationNotifier {
	return &ConfirmationNotifier{database: db, coordinator: coordinator}
}

// OnNewConfirmation implements IConfirmationNotifier
func (notifier *ConfirmationNotifier) OnNewConfirmation(ctx context.Context, coinName string) error {
	var pendingExternalTxs []TxExternal
	// query all pending external txs
	err := db.TransactionCtx(ctx, notifier.database, func(ctx context.Context, dbTx *gorm.DB) error {
		return dbTx.Model(&TxExternal{}).Joins(
			"inner join txs on txs.id = txs_external.tx_id",
		).Joins(
			"inner join wallets on txs.from_wallet_id = wallets.id",
		).Where(
			"txs.type = ? and "+
				"txs.status_id = (select id from tx_statuses where name = ?) and "+
				"wallets.coin_id = (select id from coins where short_name = ?)",
			TxTypeExternal, TxStateAwaitConfirmations, strings.ToUpper(coinName),
		).Find(&pendingExternalTxs).Error
	})
	if err != nil {
		return err
	}

	// run each confirmation query in separate goroutine
	var wg sync.WaitGroup
	wg.Add(len(pendingExternalTxs))

	// queryRes used to pass confirmation query result to this goroutine to reduce channels count
	type queryRes struct {
		err       error
		txId      int64
		confirmed bool
	}
	resChan := make(chan queryRes)
	go func() {
		wg.Wait()
		close(resChan)
	}()

	for _, tx := range pendingExternalTxs {
		go func(tx *TxExternal) {
			defer wg.Done()

			// query tx confirmation status
			confirmed, err := notifier.coordinator.TxsObserver(coinName).IsConfirmed(
				context.Background(), tx.Hash,
			)
			if err != nil {
				resChan <- queryRes{
					txId: tx.ID,
					err:  errors.Wrap(err, "error occurs while getting confirmations"),
				}
				return
			}
			resChan <- queryRes{
				txId:      tx.TxID,
				confirmed: confirmed,
			}
		}(&tx)
	}

	// gather results and errors
	var qErrs []error
	confirmedTxsIDs := make([]int64, 0, len(pendingExternalTxs))
	for res := range resChan {
		if res.err != nil {
			qErrs = append(qErrs, res.err)
		} else if res.confirmed {
			confirmedTxsIDs = append(confirmedTxsIDs, res.txId)
		}
	}
	// don't break further processing if some confirmations errors has occurred, except case when each call has returned
	// error
	if len(pendingExternalTxs) == len(qErrs) {
		return merrors.Append(nil, qErrs...)
	}
	if len(confirmedTxsIDs) == 0 {
		return nil
	}

	// update txs statuses for confirmed transactions
	err = db.TransactionCtx(ctx, notifier.database, func(ctx context.Context, dbTx *gorm.DB) error {
		// query status explicitly, no clear way with gorm :(
		var stateModel TxStatus
		err = dbTx.Model(&stateModel).Where("name = ?", TxStateProcessed).First(&stateModel).Error
		if err != nil {
			return err
		}

		return dbTx.Model(&Tx{}).Where(
			"id in ?", confirmedTxsIDs,
		).Update("StatusID", stateModel.ID).Error
	})
	if err != nil {
		return errors.Wrap(err, "error occurs while updating transactions statuses")
	}
	return nil
}
