package txs

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/types"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"github.com/jinzhu/gorm"
	"time"
)

// DateRangeFilter used to filter txs by last update or creation time. If bound value is nil, such filter will not be
// applied.
type DateRangeFilter struct {
	FromTime  *time.Time
	UntilTime *time.Time
}

// UserFilter filter txs by user phone (txs for all user wallet will be returned). Should be non empty, normalized
// form isn't required.
type UserFilter string

// WalletIDFilter filter by wallet.
type WalletIDFilter int64

// RecipientPhoneFilter filter by recipient phone number
type RecipientPhoneFilter string

// CoinFilter filter by coin name. This value is case insensitive.
type CoinFilter string

// Pager applies pager
type Pager struct {
	FromID int64
	Count  int64
}

// Filterer is visitor which applies filter conditions onto request
type Filterer interface {
	filter(ctx filterContext) (nCtx filterContext, err error)
}

// IApi this api used to access transactions
type IApi interface {
	// Get get transaction by ID
	Get(ctx context.Context, id int64) (*processing.Tx, error)

	// GetFiltered list of transactions filtered using fitlterers. If pager doesn't limit items count, then default
	// items count will be returned.
	//
	// Also returns total items count which satisfy filters conditions (except pager filter) and flag which indicates
	// is there next page available.
	GetFiltered(ctx context.Context, filters ...Filterer) (txs []processing.Tx, totalCount int64, hasNext bool, err error)
}

//
type filterContext struct {
	db                *gorm.DB
	fromWalletsJoined bool
	toWalletsJoined   bool
}

//
func (f DateRangeFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx

	if f.FromTime == nil && f.UntilTime == nil {
		return
	}
	if f.FromTime != nil && !f.FromTime.IsZero() {
		nCtx.db = nCtx.db.Where("txs.updated_at >= ?", f.FromTime)
	}

	if f.FromTime != nil && !f.FromTime.IsZero() {
		nCtx.db = nCtx.db.Where("txs.updated_at <= ?", f.FromTime)
	}
	return
}

func (f UserFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx

	phone, err := types.NewPhone(string(f))
	if err != nil {
		return
	}

	nCtx = safeJoinWallets(nCtx)
	nCtx.db = nCtx.db.Where("wallets.user_phone = ?", phone)
	return
}

func (f WalletIDFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = ctx
	nCtx.db = nCtx.db.Where("wallets.from_wallet_id = ?", f)
	return
}

func (f RecipientPhoneFilter) filter(ctx filterContext) (nCtx filterContext, err error) {
	nCtx = safeJoinWallets(ctx)
	nCtx.db = nCtx.db.Joins("inner join (select * wallets where user_phone = ?) on ")
	nCtx.db = nCtx.db.Where("wallets.user_phone = ?")
}

// safeJoinWallets applies join on query safely
func safeJoinWallets(ctx filterContext) (nCtx filterContext) {
	nCtx = ctx

	if !ctx.fromWalletsJoined {
		nCtx.db = nCtx.db.Joins("inner join wallets on txs.from_wallet_id = wallets.id")
		nCtx.fromWalletsJoined = true
	}
	return
}