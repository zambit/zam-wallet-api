package worker

import (
	"git.zam.io/wallet-backend/wallet-api/cmd/common"
	"git.zam.io/wallet-backend/wallet-api/config"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/providers"
	"git.zam.io/wallet-backend/web-api/cmd/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/dig"
	"time"
)

// Create and initialize server command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	command := cobra.Command{
		Use:   "worker",
		Short: "Runs Wallet-API misc-purposes worker",
		RunE: func(_ *cobra.Command, args []string) error {
			return serverMain(*cfg)
		},
	}
	// add common flags
	command.Flags().String(
		"db.uri",
		v.GetString("db.uri"),
		"postgres connection uri",
	)
	v.BindPFlags(command.Flags())

	return command
}

// serverMain
func serverMain(cfg config.RootScheme) (err error) {
	// create DI container and populate it with providers
	c := dig.New()

	// provide basic stuff
	common.ProvideBasic(c, cfg)

	// provide notifier
	utils.MustProvide(c, providers.CheckOutdatedNotifier)

	// Run worker
	utils.MustInvoke(c, func(logger logrus.FieldLogger, notifier processing.ICheckOutdatedNotifier) error {
		sleepTimeout := time.Hour

		l := logger.WithField("module", "wallets.worker")
		for {
			l.Debug("checking outdated")
			err := notifier.OnCheckOutdated()
			if err != nil {
				l.WithError(err).Error("error occurs while updating")
			}

			l.Debugf("sleeping for %v", sleepTimeout)
			time.Sleep(sleepTimeout)
		}
	})

	return
}
