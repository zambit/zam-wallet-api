package processing

import (
	"context"
	"encoding/json"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/jinzhu/gorm/dialects/postgres"
	"github.com/lib/pq"
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
	coinName = strings.ToUpper(coinName)

	err := notifier.watchConfirmations(ctx, coinName)
	if err != nil {
		return err
	}

	return notifier.watchNewTxs(ctx, coinName)
}

func (notifier *ConfirmationNotifier) watchConfirmations(ctx context.Context, coinName string) error {
	var pendingExternalTxs []TxExternal
	// query all pending external txs
	err := db.TransactionCtx(ctx, notifier.database, func(ctx context.Context, dbTx *gorm.DB) error {
		return dbTx.Model(&TxExternal{}).Joins(
			"inner join txs on txs.id = txs_external.tx_id",
		).Joins(
			`inner join wallets on (
				txs.from_wallet_id = wallets.id or 
				txs.to_wallet_id = wallets.id
            )`,
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
		abandoned bool
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
			confirmed, adandoned, err := notifier.coordinator.TxsObserver(coinName).IsConfirmed(
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
				abandoned: adandoned,
			}
		}(&tx)
	}

	// gather results and errors
	var qErrs []error
	confirmedTxsIDs := make([]int64, 0, len(pendingExternalTxs))
	abandonedTxsIDs := make([]int64, 0, len(pendingExternalTxs))
	for res := range resChan {
		if res.err != nil {
			qErrs = append(qErrs, res.err)
		} else if res.abandoned {
			abandonedTxsIDs = append(abandonedTxsIDs, res.txId)
		} else if res.confirmed {
			confirmedTxsIDs = append(confirmedTxsIDs, res.txId)
		}
	}
	// don't break further processing if some confirmations errors has occurred, except case when each call has returned
	// error
	if len(pendingExternalTxs) == len(qErrs) {
		return merrors.Append(nil, qErrs...)
	}
	if len(confirmedTxsIDs) == 0 && len(abandonedTxsIDs) == 0 {
		return nil
	}

	// update txs statuses for confirmed transactions
	err = db.TransactionCtx(ctx, notifier.database, func(ctx context.Context, dbTx *gorm.DB) error {
		// update confirmed transactions statuses
		if len(confirmedTxsIDs) != 0 {
			err = updateTxsStatus(dbTx, confirmedTxsIDs, TxStateProcessed)
			if err != nil {
				return err
			}
		}

		if len(abandonedTxsIDs) != 0 {
			err = updateTxsStatus(dbTx, abandonedTxsIDs, TxStateDeclined)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error occurs while updating transactions statuses")
	}
	return nil
}

func updateTxsStatus(dbTx *gorm.DB, ids []int64, newStatusName string) error {
	// query status explicitly, no clear way with gorm :(
	var stateModel TxStatus
	err := dbTx.Model(&stateModel).Where("name = ?", newStatusName).First(&stateModel).Error
	if err != nil {
		return err
	}

	return dbTx.Model(&Tx{}).Where(
		"id = ANY ($2::bigint[])", pq.Array(ids),
	).Update("StatusID", stateModel.ID).Error
}

func (notifier *ConfirmationNotifier) watchNewTxs(ctx context.Context, coinName string) error {
	// check is there is new transactions
	// TODO this block must be moved outside of here, such functionality must relies onto crypto-gate events emitter
	// rather then on such constructions
	incomingTxs, err := notifier.coordinator.TxsObserver(coinName).GetIncoming(ctx)
	if err != nil {
		return err
	}
	if len(incomingTxs) == 0 {
		return nil
	}
	incomingTxsHashes := make([]string, 0, len(incomingTxs))
	incomingTxsMap := make(map[string]nodes.IncomingTxDescr, len(incomingTxs))
	for _, tx := range incomingTxs {
		incomingTxsMap[tx.Hash] = tx
		incomingTxsHashes = append(incomingTxsHashes, tx.Hash)
	}

	err = db.TransactionCtx(ctx, notifier.database, func(ctx context.Context, dbTx *gorm.DB) error {
		// firstly select new transactions hashes which is H(ntxs) - H(atxs), where H(ntxs) - set of hashes of incoming
		// transactions and H(atxs) - set of hashes of already tracked transactions
		var newTxsHashes []struct{
			Hash string
		}
		err := dbTx.Raw(
			`select unnest($1::varchar(512)[]) as hash except select hash from txs_external`,
			pq.Array(incomingTxsHashes),
		).Scan(&newTxsHashes).Error
		if err != nil {
			return err
		}
		// skip the rest of job is there is no new txs
		if len(newTxsHashes) == 0 {
			return nil
		}

		newTxs := make([]nodes.IncomingTxDescr, 0, len(newTxsHashes))
		for _, h := range newTxsHashes {
			newTxs = append(newTxs, incomingTxsMap[h.Hash])
		}

		encoded, err := json.Marshal(&newTxs)
		if err != nil {
			return err
		}

		// create external txs and prepare list of parameters to create new external txs
		var newExternalTxs []TxExternal
		err = dbTx.Raw(
			`with values as (
  select (r->>'Address')::varchar(64) as address,
         (r->>'Amount')::decimal as amount,
         (r->>'Confirmed')::boolean as confirmed,
         (r->>'Abandoned')::boolean as abandoned,
         (r->>'Hash')::varchar(512) as tx_hash
  from json_array_elements($1) as r
),
data as (
  select
    w.id as wid,
    'external'::tx_type as t,
    v.amount as a,
    (
      case when v.abandoned then (select id from tx_statuses where name = $2)
           when v.confirmed then (select id from tx_statuses where name = $3)
           when not v.confirmed then (select id from tx_statuses where name = $4)
      end
    ) as sid,
    v.tx_hash as hash,
    v.address as addr,
    row_number() OVER (ORDER BY id) AS rn
  from wallets as w
  inner join values as v on w.address = v.address
  where w.coin_id = (select id from coins where short_name = $5)
),
inserted as (
  insert into txs (to_wallet_id, type, amount, status_id) select wid, t, a, sid from data
  returning id
)
select i.id as tx_id, d.hash as hash, d.addr as recipient
from (select id, row_number() OVER (ORDER BY id) as rn from inserted) as i
join data as d using (rn)`,
			postgres.Jsonb{RawMessage: json.RawMessage(encoded)},
			TxStateDeclined,
			TxStateProcessed,
			TxStateAwaitConfirmations,
			coinName,
		).Scan(&newExternalTxs).Error
		if err != nil {
			return err
		}

		// then create them
		for _, etx := range newExternalTxs {
			err = dbTx.Create(&etx).Error
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}