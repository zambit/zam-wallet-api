package wallets

import (
	"fmt"
	"git.zam.io/wallet-backend/common/pkg/errors"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/ericlagergren/decimal"
	"strings"
	"sync"
)

// Api provides methods to create wallets both in blockchain and db and query them
type Api struct {
	database    *db.Db
	coordinator nodes.ICoordinator
}

// NewApi create new api instance
func NewApi(d *db.Db, coordinator nodes.ICoordinator) *Api {
	return &Api{d, coordinator}
}

// CreateWallet creates wallet both in db and blockchain node and assigns actual address
func (api *Api) CreateWallet(userID int64, coinName, walletName string) (wallet WalletWithBalance, err error) {
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
				UserID: userID,
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
func (api *Api) GetWallet(userID, walletID int64) (wallet WalletWithBalance, err error) {
	err = api.database.Tx(func(tx db.ITx) error {
		wallet.Wallet, err = queries.GetWallet(tx, userID, walletID)
		return err
	})
	if err != nil {
		return
	}

	// query actual balance
	wallet.Balance, err = api.queryBalance(&wallet.Wallet)

	return
}

// GetWallets returns all wallets which belongs to a specific user applying filter and pagination params
func (api *Api) GetWallets(userID int64, onlyCoin string, fromID, count int64) (
	wts []WalletWithBalance, totalCount int64, hasNext bool, err error,
) {
	var rawWts []queries.Wallet
	err = api.database.Tx(func(tx db.ITx) error {
		rawWts, totalCount, hasNext, err = queries.GetWallets(tx, userID, queries.GetWalletFilters{
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
			wallet.Balance, err = api.queryBalance(&wallet.Wallet)
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
	var errs []error
	for queryErr := range errsChan {
		errs = append(errs, queryErr)
	}
	if errs != nil {
		err = errors.MultiErrors(errs)
		return
	}

	return
}

// ValidateCoin validates coin with given name exists
func (api *Api) ValidateCoin(coinName string) (err error) {
	_, err = queries.GetCoin(api.database, coinName)
	return
}

//
func (api *Api) queryBalance(wallet *queries.Wallet) (balance *decimal.Big, err error) {
	observer, _ := api.coordinator.Observer(wallet.Coin.ShortName)
	balance, err = observer.Balance(wallet.Address)
	return
}
