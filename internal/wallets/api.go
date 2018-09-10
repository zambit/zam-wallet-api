package wallets

import (
	"context"
	"errors"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/common/pkg/types"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/ericlagergren/decimal"
	"github.com/opentracing/opentracing-go"
	"strings"
	"sync"
)

// Api provides methods to create wallets both in blockchain and db and query them
type Api struct {
	database      *db.Db
	coordinator   nodes.ICoordinator
	processingApi processing.IApi
	balanceHelper helpers.IBalance
}

// NewApi create new api instance
func NewApi(d *db.Db, coordinator nodes.ICoordinator, processingApi processing.IApi, balanceHelper helpers.IBalance) *Api {
	return &Api{d, coordinator, processingApi, balanceHelper}
}

// CreateWallet creates wallet both in db and blockchain node and assigns actual address
func (api *Api) CreateWallet(ctx context.Context, userPhone string, coinName, walletName string) (wallet WalletWithBalance, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "creating_wallet")
	defer span.Finish()

	// uppercase coin name because everywhere coin short name used in such format
	coinName = strings.ToUpper(coinName)
	span.LogKV("user_phone", userPhone, "coin_name", coinName)

	// validate coin name
	_, err = queries.GetCoin(api.database, coinName)
	if err != nil {
		return
	}

	// coerce phone number
	userPhone, err = coercePhoneNumber(userPhone)
	if err != nil {
		return
	}

	generator := api.coordinator.Generator(coinName)

	// since we wouldn't allow an user to create multiple wallets of
	// same name here we relies onto unique user/name constraint
	// so concurrent attempt to create next wallets with duplicated pairs
	// will be locked until first occurred transaction will be committed (in such case
	// constraint violation will occurs) or rollbacked (in such case wallet will be successfully
	// inserted)
	//
	// while other transactions hungs on this call we may safely generate wallet address (we sure
	// that no concurrent call on same user/name pair will occurs between insert and update, also
	// commit will be successful)
	//
	// TODO commit may be failed due to connection issues (for example), so wallet address will be generated, but no appropriate record occurs
	err = api.database.Tx(func(tx db.ITx) (err error) {
		wallet.Wallet, err = queries.CreateWallet(
			tx, queries.Wallet{
				UserPhone: userPhone,
				Coin: queries.Coin{
					ShortName: coinName,
				},
				Name: fmt.Sprintf("%s wallet", coinName),
			},
		)
		if err != nil {
			return
		}

		// after wallet was successfully created we may generate new wallet address
		trace.InsideSpan(ctx, "wallet_generation", func(ctx context.Context, span opentracing.Span) {
			wallet.Address, err = generator.Create(ctx)
			if err != nil {
				trace.LogErrorWithMsg(span, err, "error occurs while generating wallet address")
			}
		})
		if err != nil {
			return
		}

		span.LogKV("generated_address", wallet.Address)

		// then update wallet to new address
		err = queries.UpdateWallet(tx, wallet.ID, &queries.WalletDiff{Address: &wallet.Address})

		return
	})

	if err != nil {
		return
	}

	// notify processing that wallet created
	trace.LogMsg(span, "notifying processing center that wallet created")
	err = api.processingApi.NotifyUserCreatesWallet(ctx, &wallet.Wallet)
	return
}

// GetWallet returns wallet of given id
func (api *Api) GetWallet(ctx context.Context, userPhone string, walletID int64) (wallet WalletWithBalance, err error) {
	err = trace.InsideSpanE(ctx, "getting_wallet", func(ctx context.Context, span opentracing.Span) error {
		// coerce phone number
		userPhone, err = coercePhoneNumber(userPhone)
		if err != nil {
			return err
		}

		err = api.database.Tx(func(tx db.ITx) error {
			wallet.Wallet, err = queries.GetWallet(tx, userPhone, walletID)
			return err
		})
		if err != nil {
			return err
		}

		return trace.InsideSpanE(ctx, "querying_balance", func(ctx context.Context, span opentracing.Span) error {
			// query actual balance
			var queryErr error
			wallet.Balance, queryErr = api.queryBalance(ctx, &wallet.Wallet)
			return queryErr
		})
	})
	return
}

// GetWallets returns all wallets which belongs to a specific user applying filter and pagination params
func (api *Api) GetWallets(ctx context.Context, userPhone string, onlyCoin string, fromID, count int64) (
	wts []WalletWithBalance, totalCount int64, hasNext bool, err error,
) {
	err = trace.InsideSpanE(ctx, "getting_wallets", func(ctx context.Context, span opentracing.Span) error {
		span.LogKV("user_phone", userPhone, "coin_name", onlyCoin)

		// coerce phone number
		userPhone, err := coercePhoneNumber(userPhone)
		if err != nil {
			return err
		}

		var rawWts []queries.Wallet
		err = api.database.Tx(func(tx db.ITx) error {
			rawWts, totalCount, hasNext, err = queries.GetWallets(tx, queries.GetWalletFilters{
				UserPhone: userPhone,
				ByCoin:    onlyCoin,
				FromID:    fromID,
				Count:     count,
			})
			return err
		})
		if err != nil {
			return err
		}

		wts = make([]WalletWithBalance, len(rawWts))
		// query wallets balances async in the span
		return trace.InsideSpanE(ctx, "querying_balances", func(ctx context.Context, span opentracing.Span) error {
			wg := sync.WaitGroup{}
			wg.Add(len(rawWts))
			errsChan := make(chan error)

			span.LogKV("wallets_num", len(wts))

			for i, rawWallet := range rawWts {
				// because right now amount of wallets belongs to an user ~= 3-4, it's more expediently to run goroutines
				// rather then use workers pool
				go func(i int, rawWallet queries.Wallet) {
					defer wg.Done()

					var queryErr error
					wallet := WalletWithBalance{Wallet: rawWallet}
					wallet.Balance, queryErr = api.queryBalance(ctx, &wallet.Wallet)
					if queryErr != nil {
						errsChan <- queryErr
						return
					}
					wts[i] = wallet
				}(i, rawWallet)
			}

			// wait until all jobs done in separated goroutine
			go func() {
				wg.Wait()
				close(errsChan)
			}()

			//
			for queryErr := range errsChan {
				err = merrors.Append(err, queryErr)
			}
			return err
		})
	})
	return
}

// ValidateCoin validates coin with given name exists
func (api *Api) ValidateCoin(coinName string) (err error) {
	_, err = queries.GetCoin(api.database, coinName)
	return
}

// SendToPhone sends internal transaction determining recipient wallet by source wallet and dest phone number. If
// user not exists, transaction will be marked as "pending" and may be continued by `NotifyUserCreatesWallet` call.
// May return ErrNoSuchWallet.
func (api *Api) SendToPhone(
	ctx context.Context,
	userPhone string,
	walletID int64,
	toUserPhone string,
	amount *decimal.Big,
) (newTx *processing.Tx, err error) {
	err = trace.InsideSpanE(ctx, "send_to_phone", func(ctx context.Context, span opentracing.Span) error {
		span.LogKV("user_phone", userPhone, "wallet_id", walletID, "to_user_phone", toUserPhone)

		// coerce user phone number
		userPhone, err = coercePhoneNumber(userPhone)
		if err != nil {
			return err
		}

		// gather validation errors
		var validationErrs error
		// coerce recipient phone number
		toUserPhone, err = coercePhoneNumber(toUserPhone)
		if err != nil {
			validationErrs = merrors.Append(validationErrs, err)
		}

		// check amount
		if amount.Sign() <= 0 {
			validationErrs = merrors.Append(validationErrs, errs.ErrNonPositiveAmount)
		}

		// forbid self transactions
		if userPhone == toUserPhone {
			validationErrs = merrors.Append(validationErrs, errs.ErrSelfTxForbidden)
		}

		if validationErrs != nil {
			return validationErrs
		}

		var (
			fromWallet   queries.Wallet
			candidate    processing.TxRecipientCandidate
			fbCandidates []processing.TxRecipientCandidate
		)
		err = api.database.Tx(func(tx db.ITx) (err error) {
			// query source wallet
			fromWallet, err = queries.GetWallet(tx, userPhone, walletID)
			if err != nil {
				return
			}

			// lookup destination user wallet
			wts, _, _, err := queries.GetWallets(
				tx,
				queries.GetWalletFilters{UserPhone: toUserPhone, ByCoin: fromWallet.Coin.ShortName},
			)
			if err != nil {
				return err
			}

			span.LogKV("dst_wallets_num", len(wts))

			switch len(wts) {
			case 0:
				candidate = processing.NewPhoneRecipient(toUserPhone)
				trace.LogMsg(span, "sending by phone due to recipient wallet not found")
			case 1:
				candidate = processing.NewWalletRecipient(&wts[0])
				fbCandidates = append(fbCandidates, processing.NewAddressRecipient(wts[0].Address))
				span.LogKV("dst_wallet_id", wts[0].ID)
				trace.LogMsg(span, "sending to dst wallet")
			default:
				// we doesn't support multiple wallets of same coin which belongs to one user
				return errors.New("wallets: not implemented: user same coin multi wallet not supported")
			}

			return nil
		})
		if err != nil {
			trace.LogErrorWithMsg(span, err, "error occurs before sending")
			return err
		}

		return trace.InsideSpanE(ctx, "sending", func(ctx context.Context, span opentracing.Span) error {
			var sendErr error
			newTx, sendErr = api.processingApi.Send(
				ctx,
				&fromWallet,
				candidate,
				amount,
				fbCandidates...,
			)
			return sendErr
		})
	})
	return
}

// SentToAddress
func (api *Api) SentToAddress(
	ctx context.Context,
	userPhone string,
	walletID int64,
	toAddress string,
	amount *decimal.Big,
) (newTx *processing.Tx, err error) {
	err = trace.InsideSpanE(ctx, "send_to_address", func(ctx context.Context, span opentracing.Span) error {
		var fromWallet queries.Wallet

		span.LogKV("user_phone", userPhone, "wallet_id", walletID, "to_address", toAddress, "amount", amount)

		// coerce user phone number
		userPhone, err = coercePhoneNumber(userPhone)
		if err != nil {
			return err
		}

		// check amount
		if amount.Sign() <= 0 {
			return errs.ErrNonPositiveAmount
		}

		// decide recipient type: if an a wallet of such coin and destination address exists, hint suggest processing
		// to use that
		recipient := processing.NewAddressRecipient(toAddress)
		var fbRecipients []processing.TxRecipientCandidate
		err = api.database.Tx(func(tx db.ITx) error {
			var err error
			// query source wallet
			fromWallet, err = queries.GetWallet(tx, userPhone, walletID)
			if err != nil {
				return err
			}

			// first we need to check is that address belongs to some recipientWallet in the system so this transaction
			// will be sent internally
			// TODO this decision must be made inside processing but currently due to DDD principe this check stay here
			wts, _, _, err := queries.GetWallets(
				tx,
				queries.GetWalletFilters{ByCoin: fromWallet.Coin.ShortName, ByAddress: toAddress},
			)
			if err != nil {
				return err
			}

			// if some wallets found, use them to send internal tx
			if len(wts) > 0 {
				span.LogKV("to_wallet_id", wts[0].ID, "to_wallet_user_phone", wts[0].UserPhone)
				if wts[0].UserPhone == userPhone {
					return errs.ErrSelfTxForbidden
				}
				recipient = processing.NewWalletRecipient(&wts[0])
				// also provide fallback address recipients
				fbRecipients = append(fbRecipients, processing.NewAddressRecipient(toAddress))
			}
			return nil
		})
		return trace.InsideSpanE(ctx, "sending", func(ctx context.Context, span opentracing.Span) error {
			var sendErr error
			newTx, sendErr = api.processingApi.Send(ctx, &fromWallet, recipient, amount, fbRecipients...)
			return sendErr
		})
	})
	return
}

//
func (api *Api) queryBalance(ctx context.Context, wallet *queries.Wallet) (balance *decimal.Big, err error) {
	return api.balanceHelper.TotalWalletBalanceCtx(ctx, wallet)
}

func coercePhoneNumber(userPhone string) (string, error) {
	userPhoneParsed, err := types.NewPhone(userPhone)
	if err != nil {
		err = errs.ErrInvalidPhone
		return "", err
	}
	return string(userPhoneParsed), nil
}
