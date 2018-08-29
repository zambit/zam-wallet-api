package watcher

import (
	"context"
	"git.zam.io/wallet-backend/wallet-api/cmd/common"
	"git.zam.io/wallet-backend/wallet-api/config"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/providers"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	_ "git.zam.io/wallet-backend/wallet-api/internal/services/nodes/btc"
	"git.zam.io/wallet-backend/web-api/cmd/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/dig"
	"strings"
)

// Create and initialize server command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	var coinName string
	command := cobra.Command{
		Use:       "watcher [coin_name]",
		Short:     "Runs Wallet-API block-chain watcher",
		ValidArgs: []string{"coin"},
		Args:      cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			coinName = strings.ToLower(coinName)
			return watcherMain(*cfg, args[0])
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
func watcherMain(cfg config.RootScheme, coinName string) (err error) {
	if coinName == "" {
		return errors.New("watcher: empty coin name	")
	}

	// create DI container and populate it with providers
	c := dig.New()

	// provide basic stuff
	common.ProvideBasic(c, cfg)

	// provide notifier
	utils.MustProvide(c, providers.ConfirmationsNotifier)

	// Run worker
	utils.MustInvoke(c, func(coordinator nodes.ICoordinator, notifier processing.IConfirmationNotifier) error {
		ctx := context.Background()

		loop, err := coordinator.WatcherLoop(coinName)
		if err != nil {
			return err
		}
		loop.OnNewBlockReleased(func(ctx context.Context, blockHeight int) error {
			return notifier.OnNewConfirmation(ctx, coinName)
		})
		return loop.Run(ctx)
	})

	return
}
