package txs

import (
	"context"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/common"
	walletshandlers "git.zam.io/wallet-backend/wallet-api/internal/server/handlers/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/txs"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/wallet-api/pkg/server/middlewares"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	// sending errors
	errInsufficientFunds = base.ErrorView{
		Code:    http.StatusBadRequest,
		Message: "insufficient funds",
	}
	errWrongTxAmount           = base.NewFieldErr("body", "amount", "must be greater then zero")
	errTxAmountToBig           = base.NewFieldErr("body", "amount", "such a great value can not be accepted")
	errNoSuchWallet            = base.NewFieldErr("body", "wallet_id", "no such wallet")
	errRecipientIsYou          = base.NewFieldErr("body", "recipient", "you can't send amount to your self")
	errRecipientPhoneInvalid   = base.NewFieldErr("body", "recipient", "invalid recipient phone")
	errRecipientAddressInvalid = base.NewFieldErr("body", "recipient", "invalid recipient address")

	// get tx errors
	errTxIdInvalid = base.NewFieldErr("path", "tx_id", "tx id is invalid")
	errTxNotFound  = base.NewFieldErr("path", "tx_id", "no such tx")

	// get all filters errors
	errInvalidWalletID   = base.NewFieldErr("query", "wallet_id", "invalid wallet id")
	errInvalidPage       = base.NewFieldErr("query", "page", "invalid page identifier")
	errInvalidRecipient  = base.NewFieldErr("query", "recipient", "invalid recipient phone")
	errInvalidCoinName   = base.NewFieldErr("query", "coin", "invalid coin name")
	errInvalidStatusName = base.NewFieldErr("query", "status", "invalid tx status name")
)

// SendFactory
func SendFactory(walletApi *wallets.Api, converter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		params := SendRequest{}
		err = base.ShouldBindJSON(c, &params)
		if err != nil {
			return
		}
		// bind query params ignore error
		queryParams := ConvertParams{}
		c.ShouldBindQuery(&queryParams)

		span.LogKV(
			"wallet_id", params.WalletID,
			"recipient", params.Recipient,
			"amount", params.Amount,
		)

		// extract user phone
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		var tx *processing.Tx
		if isCryproAddress(params.Recipient) {
			err = trace.InsideSpanE(ctx, "send_to_address", func(ctx context.Context, span opentracing.Span) error {
				var err error
				tx, err = walletApi.SentToAddress(
					ctx,
					userPhone,
					params.WalletID,
					params.Recipient,
					(*decimal.Big)(params.Amount),
				)
				return err
			})
		} else {
			err = trace.InsideSpanE(ctx, "send_to_phone", func(ctx context.Context, span opentracing.Span) error {
				var err error
				// try send money
				tx, err = walletApi.SendToPhone(ctx, userPhone, params.WalletID, params.Recipient, (*decimal.Big)(params.Amount))
				return err
			})
		}
		if err != nil {
			err = coerceProcessingErrs(err)
			return
		}

		// query rates ignore error
		rates, _ := getRateForTx(ctx, tx, queryParams.Convert, converter)

		// render response converting db format into api format
		resp = SingleResponse{Transaction: ToView(tx, userPhone, rates)}
		return
	}
}

// GetFactory creates get user tx by id handler, requires tx_id param in request path
func GetFactory(txsApi txs.IApi, converter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		// bind query params ignore error
		params := ConvertParams{}
		c.ShouldBindQuery(&params)

		// parse wallet id path param
		txID, txIDValid := FromIdView(c.Param("tx_id"))
		if !txIDValid {
			err = errTxIdInvalid
			return
		}
		span.LogKV("tx_id", txID)

		// extract user phone
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		var tx *processing.Tx
		err = trace.InsideSpanE(ctx, "get_tx", func(ctx context.Context, span opentracing.Span) error {
			var err error
			// get tx using txs api
			tx, err = txsApi.Get(ctx, txID, userPhone)
			return err
		})
		if err != nil {
			if err == txs.ErrNoSuchTx {
				err = errTxNotFound
			}
			return
		}

		// query rates ignore error
		rates, _ := getRateForTx(ctx, tx, params.Convert, converter)

		// prepare response body
		resp = SingleResponse{Transaction: ToView(tx, userPhone, rates)}
		return
	}
}

const defaultTxCountValue = 20

// GetAllFactory creates get all user txs request handler
func GetAllFactory(txsApi txs.IApi, converter convert.ICryptoCurrency) base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		span, ctx := trace.GetSpanWithCtx(c)
		defer span.Finish()

		// parse query params
		params := GetAllRequest{}

		// TODO now params which follows invalid param will not be populated. Rework whole bind/validation layer
		c.ShouldBindQuery(&params)
		var (
			groupTxs bool
			groupTZ  *time.Location
		)
		if params.Group != "" {
			params.Group = strings.ToLower(params.Group)
			switch params.Group {
			case "hour", "day", "week", "month":
				groupTxs = true
			default:
				// ignore invalid param
				params.Group = ""
			}

			// process timezone parameter only if groupping is required
			if groupTxs && params.Timezone != "" {
				offsetHours, pErr := strconv.ParseFloat(params.Timezone, 10)
				// ignore error, also check that abs offset less then 12 hours
				if pErr == nil && offsetHours >= -12 && offsetHours <= 12 {
					groupTZ = time.FixedZone("", int(offsetHours*60*60))
				}
			}
		}

		// extract user phone
		userPhone, err := middlewares.GetUserPhoneFromCtxE(c)
		if err != nil {
			return
		}
		span.LogKV("user_phone", userPhone)

		// build filters by query params
		filters, err := generateFilters(params, userPhone)
		if err != nil {
			return
		}

		var (
			allTxs     []processing.Tx
			totalCount int64
			hasNext    bool
		)
		err = trace.InsideSpanE(ctx, "get_txs", func(ctx context.Context, span opentracing.Span) error {
			var err error
			// get tx using txs api
			allTxs, totalCount, hasNext, err = txsApi.GetFiltered(ctx, filters...)
			return err
		})
		if err != nil {
			// coerce filters validation errors
			switch err {
			case txs.ErrInvalidRecipientPhone:
				err = errInvalidRecipient
			case txs.ErrInvalidCoinName:
				err = errInvalidCoinName
			case txs.ErrInvalidStatus:
				err = errInvalidStatusName
			}
			return
		}

		// get rates ignore error
		rates, _ := getRatesForTxs(ctx, allTxs, params.Convert, converter, common.DefaultCryptoCurrency)

		var next *string
		if hasNext && len(allTxs) > 0 {
			t := ToIdView(allTxs[len(allTxs)-1].ID)
			next = &t
		}
		count := len(allTxs)

		if !groupTxs {
			// prepare response body for ungrounded txs
			resp = MultipleResponse{
				Transactions: ToAllView(allTxs, userPhone, rates),
				TotalCount:   totalCount,
				Count:        count,
				Next:         next,
			}
		} else {
			// prepare response for grouped txs
			resp = GroupedResponse{
				GroupedTransactions: ToGroupViews(allTxs, userPhone, rates, params.Group, groupTZ),
				TotalCount:          totalCount,
				Count:               count,
				Next:                next,
			}
		}
		return
	}
}

func coerceProcessingErrs(err error) error {
	if errors, ok := err.(merrors.Errors); ok {
		for i, e := range errors {
			errors[i] = coerceProcessingErr(e)
		}
		return errors
	}
	return coerceProcessingErr(err)
}

func coerceProcessingErr(e error) (newE error) {
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
	case errs.ErrInvalidPhone:
		newE = errRecipientPhoneInvalid
	case processing.ErrInvalidAddress:
		newE = errRecipientAddressInvalid
	default:
		newE = e
	}
	return
}

// isBtcAddress tries to guess is that sting represents btc address, address format description taken from here
// https://en.bitcoinwiki.org/wiki/Bitcoin_address
// Bitcoin address is an identifier (account number), starting with 1 or 3 or bc1 and containing 27-34 alphanumeric
// Latin characters (except 0, O, I)
func isBtcAddress(candidate string) bool {
	if len(candidate) >= 27 && len(candidate) <= 42 {
		return true
	}

	return false
}

// isCryproAddress looks is candidate looks like crypto-address
func isCryproAddress(candidate string) bool {
	return isBtcAddress(candidate)
}

// generates txs filters params for specified user
func generateFilters(params GetAllRequest, userPhone string) (filters []txs.Filterer, err error) {
	filters = make([]txs.Filterer, 0, 2)
	// apply user phone filter
	filters = append(filters, txs.UserFilter(userPhone))
	// apply coin filter
	if params.Coin != nil {
		filters = append(filters, txs.CoinFilter(*params.Coin))
	}
	// apply status filter
	if params.Status != nil {
		filters = append(filters, txs.StatusFilter(*params.Status))
	}
	// apply wallet id filter
	if params.WalletID != nil {
		walletID, valid := walletshandlers.ParseWalletIDView(*params.WalletID)
		if !valid {
			err = errInvalidWalletID
			return
		}
		filters = append(filters, txs.WalletIDFilter(walletID))
	}
	// apply recipient filter
	if params.Recipient != nil {
		filters = append(filters, txs.RecipientPhoneFilter(*params.Recipient))
	}
	// apply time range filters
	if params.FromTime != nil || params.UntilTime != nil {
		var (
			from *time.Time
			to   *time.Time
		)
		// TODO find gin query bind workaround for unix timestamps
		if params.FromTime != nil {
			fromTs, pErr := strconv.ParseInt(*params.FromTime, 10, 64)
			if pErr == nil {
				t := time.Unix(fromTs, 0).UTC()
				from = &t
			}
		}
		if params.UntilTime != nil {
			toTs, pErr := strconv.ParseInt(*params.UntilTime, 10, 64)
			if pErr == nil {
				t := time.Unix(toTs, 0).UTC()
				to = &t
			}
		}
		filters = append(filters, txs.DateRangeFilter{FromTime: from, UntilTime: to})
	}
	if params.Direction != nil {
		switch *params.Direction {
		case "incoming":
			filters = append(filters, txs.DirectionFilter(true))
		case "outgoing":
			filters = append(filters, txs.DirectionFilter(false))
		}
		// ignore invalid values
	}
	// apply pagination
	// it will be applied despite of incoming params because there is default rows limit
	{
		pager := txs.Pager{}
		if params.Count != nil {
			pager.Count = *params.Count
		} else {
			// by default limit response items
			pager.Count = defaultTxCountValue
		}
		if params.Page != nil {
			page, valid := FromIdView(*params.Page)
			if !valid {
				err = errInvalidPage
				return
			}
			pager.FromID = page
		}
		filters = append(filters, &pager)
	}
	return
}
