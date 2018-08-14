package wallets

import (
	"context"
	"errors"
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/merrors"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/ericlagergren/decimal"
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
	// uppercase coin name because everywhere coin short name used in such format
	coinName = strings.ToUpper(coinName)

	// validate name name
	_, err = queries.GetCoin(api.database, coinName)
	if err != nil {
		return
	}

	// validate name and get generator for specific name using coordinator
	generator, err := api.coordinator.Generator(coinName)
	if err != nil {
		return
	}

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
		wallet.Address, err = generator.Create()
		if err != nil {
			return
		}

		// then update wallet to new address
		err = queries.UpdateWallet(tx, wallet.ID, &queries.WalletDiff{Address: &wallet.Address})

		return
	})
	return
}

// GetWallet returns wallet of given id
func (api *Api) GetWallet(ctx context.Context, userPhone string, walletID int64) (wallet WalletWithBalance, err error) {
	err = api.database.Tx(func(tx db.ITx) error {
		wallet.Wallet, err = queries.GetWallet(tx, userPhone, walletID)
		return err
	})
	if err != nil {
		return
	}

	// query actual balance
	wallet.Balance, err = api.queryBalance(ctx, &wallet.Wallet)

	return
}

// GetWallets returns all wallets which belongs to a specific user applying filter and pagination params
func (api *Api) GetWallets(ctx context.Context, userPhone string, onlyCoin string, fromID, count int64) (
	wts []WalletWithBalance, totalCount int64, hasNext bool, err error,
) {
	var rawWts []queries.Wallet
	err = api.database.Tx(func(tx db.ITx) error {
		rawWts, totalCount, hasNext, err = queries.GetWallets(tx, userPhone, queries.GetWalletFilters{
			ByCoin: onlyCoin,
			FromID: fromID,
			Count:  count,
		})
		return err
	})
	if err != nil {
		return
	}
	// query wallets balances async
	wg := sync.WaitGroup{}
	wg.Add(len(rawWts))
	errsChan := make(chan error)

	wts = make([]WalletWithBalance, len(rawWts))
	for i, rawWallet := range rawWts {
		// because right now amount of wallets belongs to an user ~= 3-4, it's more expediently to run goroutines
		// rather then use workers pool
		go func(i int, rawWallet queries.Wallet) {
			defer wg.Done()

			var err error
			wallet := WalletWithBalance{Wallet: rawWallet}
			wallet.Balance, err = api.queryBalance(ctx, &wallet.Wallet)
			if err != nil {
				errsChan <- err
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
func (api *Api) SendToPhone(ctx context.Context, userPhone string, walletID int64, toUserPhone string, amount *decimal.Big) (
	newTx *processing.Tx, err error,
) {
	var (
		fromWallet queries.Wallet
		toWallet   queries.Wallet
	)
	err = api.database.Tx(func(tx db.ITx) (err error) {
		// query source wallet
		fromWallet, err = queries.GetWallet(tx, userPhone, walletID)
		if err != nil {
			return
		}

		// lookup destination user wallet
		wts, _, _, err := queries.GetWallets(tx, toUserPhone, queries.GetWalletFilters{ByCoin: fromWallet.Coin.ShortName})
		if err != nil {
			return err
		}
		// there is no special error which indicates that no wallets found due to unexisted user, so check by slice len
		if len(wts) == 0 {
			// TODO implementme: currently user wallet awaiting not supported, return error
			err = errors.New("wallets: not implemented: user wallet awaiting not implemented")
			return
		}
		if len(wts) > 1 {
			// also we doesn't support multiple wallets of same coin which belongs to one user
			err = errors.New("wallets: not implemented: user same coin multi wallet not supported")
			return
		}
		toWallet = wts[0]

		return
	})
	if err != nil {
		return
	}

	newTx, err = api.processingApi.SendInternal(ctx, &fromWallet, processing.InternalTxRecipient{Wallet: &toWallet}, amount)
	return
}

//
func (api *Api) queryBalance(ctx context.Context, wallet *queries.Wallet) (balance *decimal.Big, err error) {
	return api.balanceHelper.TotalWalletBalanceCtx(ctx, wallet)
}
