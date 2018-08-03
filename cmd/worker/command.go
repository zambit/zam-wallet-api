package worker

import (
	"git.zam.io/wallet-backend/wallet-api/cmd/common"
	"git.zam.io/wallet-backend/wallet-api/config"
	"git.zam.io/wallet-backend/wallet-api/internal/isc/handlers/users"
	_ "git.zam.io/wallet-backend/wallet-api/internal/services/nodes/btc"
	"git.zam.io/wallet-backend/web-api/cmd/utils"
	"git.zam.io/wallet-backend/web-api/pkg/services/broker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/dig"
)

// Create and initialize server command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	command := cobra.Command{
		Use:   "worker",
		Short: "Runs Wallet-API worker",
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

	// register worker event handlers
	utils.MustInvoke(c, users.Register)

	// Run worker
	utils.MustInvoke(c, func(broker broker.IBroker) error {
		err := broker.Start()
		if err != nil {
			return err
		}
		select {}
		return nil
	})

	return
}
