package users

import (
	"git.zam.io/wallet-backend/common/pkg/errors"
	"git.zam.io/wallet-backend/wallet-api/internal/isc/handlers/base"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/queries"
	"git.zam.io/wallet-backend/web-api/db"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
	"github.com/sirupsen/logrus"
)

// RegistrationCompletedFactory handles user registration event and creates wallets for all available coins
func RegistrationCompletedFactory(d *db.Db, api *wallets.Api, logger logrus.FieldLogger) base.HandlerFunc {
	logger = logger.WithField("module", "isc.users.handlers")

	return func(identifier broker.Identifier, dataBinder func(dst interface{}) error) (out base.HandlerOut, err error) {
		logger.Info("receiving user created event")

		// bind params
		params := CreatedEvent{}
		err = dataBinder(&params)
		if err != nil {
			logger.WithError(err).Info("message parsing failed")
			return
		}

		logger = logger.WithField("user_id", params.UserID)

		// query available coins and create set
		coins, err := queries.GetDefaultCoins(d)
		if err != nil {
			logger.WithError(err).Error("default coins fetch failed")
			return
		}
		coinsNamesSet := make(map[string]struct{})
		for _, c := range coins {
			coinsNamesSet[c.ShortName] = struct{}{}
		}

		// query already created wallets
		wts, _, _, err := api.GetWallets(params.UserID, "", 0, 0)
		if err != nil {
			logger.WithError(err).Error("user wallets query failed")
			return
		}

		// exclude already created wallets from coins set
		for _, w := range wts {
			if _, ok := coinsNamesSet[w.Coin.ShortName]; ok {
				delete(coinsNamesSet, w.Coin.ShortName)
			}
		}

		// create wallets for all enabled coins
		var errs []error
		for _, c := range coins {
			// force default wallet name
			_, cErr := api.CreateWallet(params.UserID, c.ShortName, "")
			if cErr != nil {
				logger.WithError(cErr).WithField("coin_name", c.ShortName).Error("wallet creation failed")
				errs = append(errs, cErr)
			}
		}
		if errs != nil && len(errs) > 0 {
			err = errors.MultiErrors(errs)
		}

		return
	}
}
