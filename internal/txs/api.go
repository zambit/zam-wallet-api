package txs

import (
	"context"
	"errors"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"github.com/jinzhu/gorm"
	"time"
)

// ErrNoSuchTx returned when no wallet with such id found
var ErrNoSuchTx = errors.New("txs: no tx with such id found")

// DateRangeFilter used to filter txs by last update or creation time. If bound value is nil, such filter will not be
// applied.
type DateRangeFilter struct {
	FromTime  *time.Time
	UntilTime *time.Time
}

// UserFilter filter txs by user phone (txs for all user wallet will be returned). Should be non empty, normalized
// form isn't required. If value is wrong, ErrInvalidUserPhone will be returned
type UserFilter string

// ErrInvalidUserPhone when user phone is invalid
var ErrInvalidUserPhone = errors.New("txs: invalid user phone")

// StatusFilter filter txs by their statuses. Should be non empty. If value is wrong, ErrInvalidStatus will be returned
type StatusFilter string

// ErrInvalidUserPhone when user phone is invalid
var ErrInvalidStatus = errors.New("txs: invalid status")

// WalletIDFilter filter by wallet.
type WalletIDFilter int64

// RecipientPhoneFilter filter by recipient phone number. Should be non empty, normalized form isn't required. If value
// is wrong, ErrInvalidRecipientPhone will be returned
type RecipientPhoneFilter string

// ErrInvalidRecipientPhone when recipient phone is invalid
var ErrInvalidRecipientPhone = errors.New("txs: recipient phone is invalid")

// CoinFilter filter by coin name. This value is case insensitive.
type CoinFilter string

// ErrInvalidCoinName returned when specified coin name is invalid
var ErrInvalidCoinName = errors.New("txs: invalid coin name")

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
	// Get transaction by ID. Also accept optional user phone argument, if it's provided, then tx will be returned
	// only for specified user. If provided value is invalid, ErrInvalidUserPhone will be returned. If
	// no rows found, then ErrNoSuchTx will be returned.
	Get(ctx context.Context, id int64, restrictUserPhone ...string) (*processing.Tx, error)

	// GetFiltered list of transactions filtered using fitlterers. If pager doesn't limit items count, then default
	// items count will be returned.
	//
	// Also returns total items count which satisfy filters conditions (except pager filter) and flag which indicates
	// is there next page available.
	GetFiltered(ctx context.Context, filters ...Filterer) (txs []processing.Tx, totalCount int64, hasNext bool, err error)
}

//
type filterContext struct {
	tx                *gorm.DB
	q                 *gorm.DB
	fromWalletsJoined bool
	toWalletsJoined   bool
}
