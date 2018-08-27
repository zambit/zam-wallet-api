package txs

import (
	"context"
	"database/sql"
	"git.zam.io/wallet-backend/common/pkg/types"
	"git.zam.io/wallet-backend/wallet-api/db"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"github.com/jinzhu/gorm"
	"strings"
)

// Api is IApi implementation
type Api struct {
	db *gorm.DB
}

// New creates new txs api
func New(db *gorm.DB) IApi {
	return &Api{db: db}
}

// Get implements IApi interface
func (api *Api) Get(ctx context.Context, id int64, restrictUserPhone ...string) (tx *processing.Tx, err error) {
	err = db.TransactionCtx(ctx, api.db, func(ctx context.Context, dbTx *gorm.DB) error {
		q := dbTx.Model(&processing.Tx{}).Where("txs.id = ?", id)

		// reuse user filter if user phone restriction is required
		if len(restrictUserPhone) > 0 {
			fCtx, err := UserFilter(restrictUserPhone[0]).filter(filterContext{tx: dbTx, q: q})
			if err != nil {
				return err
			}
			q = fCtx.q
		}

		// query tx
		tx = new(processing.Tx)
		err := addTxPreloads(q).First(tx).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				err = ErrNoSuchTx
			}
			return err
		}
		return nil
	})
	return
}

// GetFiltered implements IApi interface
func (api *Api) GetFiltered(ctx context.Context, filters ...Filterer) (txs []processing.Tx, totalCount int64, hasNext bool, err error) {
	err = db.TransactionCtx(ctx, api.db, func(ctx context.Context, dbTx *gorm.DB) error {
		q := dbTx.Model(&processing.Tx{})
		var qWOPagination *gorm.DB

		var firstItemID int64
		if len(filters) > 0 {
			var paginator *Pager

			// apply filters sequentially
			fCtx := filterContext{tx: dbTx, q: q}
			for _, f := range filters {
				// apply paginator in last order to have ability to query id of last item with same filters
				if p, ok := f.(*Pager); ok {
					paginator = p
					continue
				}

				fCtx, err = f.filter(fCtx)
				if err != nil {
					return err
				}
			}

			// save query WO pagination
			qWOPagination = fCtx.q

			// if paginator applied determine first item id and total items count
			if paginator != nil {
				var firstTx processing.Tx
				// be sure only id of first tx is queried
				err = fCtx.q.Select("txs.id").First(&firstTx).Error
				if err != nil {
					// if no rows error occurs, that mean that no txs found which satisfies specified params, so
					// we will return without error
					if err == gorm.ErrRecordNotFound {
						err = nil
					}
					return err
				}
				firstItemID = firstTx.ID

				fCtx, err = paginator.filter(fCtx)
				if err != nil {
					return err
				}
			}

			q = fCtx.q
		}

		// determine txs count
		err = qWOPagination.Count(&totalCount).Error
		if err != nil {
			return err
		}
		// no further steps is required if there is no records with given conditions
		if totalCount == 0 {
			return nil
		}

		// loads transactions
		// TODO this scan may be highly optimized if we would explicitly specify list of returned columns
		err = addTxPreloads(q).Order("txs.created_at desc").Find(&txs).Error
		if err != nil {
			return err
		}
		if len(txs) == 0 {
			return nil
		}

		// determine if there more rows available after this page
		if firstItemID != 0 {
			hasNext = firstItemID < txs[len(txs)-1].ID
		}

		return err
	})
	return
}

// addTxPreloads adds tx relations preload request to the query
func addTxPreloads(q *gorm.DB) *gorm.DB {
	return q.Preload(
		"FromWallet",
	).Preload(
		"FromWallet.Coin",
	).Preload(
		"ToWallet",
	).Preload(
		"Status",
	)
}

//
func (f DateRangeFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx

	if f.FromTime == nil && f.UntilTime == nil {
		return
	}
	if f.FromTime != nil && !f.FromTime.IsZero() {
		nCtx.q = nCtx.q.Where("txs.created_at >= ?", f.FromTime)
	}

	if f.UntilTime != nil && !f.UntilTime.IsZero() {
		nCtx.q = nCtx.q.Where("txs.created_at <= ?", f.UntilTime)
	}
	return
}

func (f UserFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	phone, err := types.NewPhone(string(f))
	if err != nil {
		err = ErrInvalidUserPhone
		return
	}

	nCtx = joinFromWalletsOnce(ctx)
	nCtx = joinToWalletsOnce(nCtx)
	nCtx.q = nCtx.q.Where("wallets.user_phone = ? or to_wallets.user_phone = ?", phone, phone)
	return
}

func (f WalletIDFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx
	nCtx.q = nCtx.q.Where("(txs.from_wallet_id = ? or txs.to_wallet_id = ?)", f, f)
	return
}

func (f RecipientPhoneFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	phone, err := types.NewPhone(string(f))
	if err != nil {
		err = ErrInvalidRecipientPhone
		return
	}

	nCtx = joinToWalletsOnce(ctx)
	nCtx.q = nCtx.q.Where("(to_wallets.user_phone = ? or txs.to_phone = ?)", phone, phone)
	return
}

func (f CoinFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	// coin name check is explicit call
	coin := strings.ToUpper(string(f))
	var count int
	err = ctx.tx.Raw(`select count(*) from coins where short_name = ?`, coin).Row().Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrInvalidCoinName
		}
		return
	}
	if count == 0 {
		err = ErrInvalidCoinName
		return
	}

	nCtx = joinFromWalletsOnce(ctx)
	nCtx.q = nCtx.q.Where("wallets.coin_id = (select id from coins where short_name = ?)", string(f))
	return
}

func (f StatusFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	// coin name check is explicit call
	coin := strings.ToLower(string(f))
	var count int
	err = ctx.tx.Raw(`select count(*) from tx_statuses where name = ?`, coin).Row().Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrInvalidStatus
		}
		return
	}
	if count == 0 {
		err = ErrInvalidStatus
		return
	}

	nCtx = ctx
	nCtx.q = nCtx.q.Where("txs.status_id = (select id from tx_statuses where name = ?)", string(f))
	return
}

func (f *Pager) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx
	if f.Count != 0 {
		nCtx.q = nCtx.q.Limit(f.Count)
	}
	if f.FromID != 0 {
		nCtx.q = nCtx.q.Where("txs.id < ?", f.FromID)
	}
	return
}

// joinFromWalletsOnce applies join on query safely
func joinFromWalletsOnce(ctx filterContext) (nCtx filterContext) {
	nCtx = ctx

	if !ctx.fromWalletsJoined {
		nCtx.q = nCtx.q.Joins("inner join wallets on txs.from_wallet_id = wallets.id")
		nCtx.fromWalletsJoined = true
	}
	return
}

// joinFromWalletsOnce applies join on query safely
func joinToWalletsOnce(ctx filterContext) (nCtx filterContext) {
	nCtx = ctx

	if !ctx.toWalletsJoined {
		nCtx.q = nCtx.q.Joins("left outer join wallets as to_wallets on txs.to_wallet_id = to_wallets.id")
		nCtx.toWalletsJoined = true
	}
	return
}
