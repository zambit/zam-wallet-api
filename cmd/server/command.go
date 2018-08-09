package server

import (
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/cmd/common"
	"git.zam.io/wallet-backend/wallet-api/config"
	internalproviders "git.zam.io/wallet-backend/wallet-api/internal/providers"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/txs"
	"git.zam.io/wallet-backend/wallet-api/internal/server/handlers/wallets"
	_ "git.zam.io/wallet-backend/wallet-api/internal/services/nodes/btc"
	"git.zam.io/wallet-backend/web-api/cmd/utils"
	"git.zam.io/wallet-backend/web-api/pkg/providers"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/static"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/dig"
)

// Create and initialize server command for given viper instance
func Create(v *viper.Viper, cfg *config.RootScheme) cobra.Command {
	command := cobra.Command{
		Use:   "server",
		Short: "Runs Wallet-API server",
		RunE: func(_ *cobra.Command, args []string) error {
			return serverMain(*cfg)
		},
	}
	// add common flags
	command.Flags().StringP("server.host", "l", v.GetString("server.host"), "host to serve on")
	command.Flags().IntP("server.port", "p", v.GetInt("server.port"), "port to serve on")
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

	// provide common stuff
	common.ProvideBasic(c, cfg)

	// provide gin engine
	utils.MustProvide(c, providers.GinEngine)

	// provide api router
	utils.MustProvide(c, providers.RootRouter, dig.Name("root"))
	utils.MustProvide(c, internalproviders.ApiRoutes, dig.Name("api_routes"))

	// provide middlewares
	utils.MustProvide(c, providers.AuthMiddleware, dig.Name("auth_middleware"))
	utils.MustProvide(c, internalproviders.UserMiddleware, dig.Name("user_middleware"))

	// register handlers
	utils.MustInvoke(c, static.Register)
	utils.MustInvoke(c, wallets.Register)
	utils.MustInvoke(c, txs.Register)

	// Run server!
	utils.MustInvoke(c, func(engine *gin.Engine) error {
		return engine.Run(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
	})

	return
}
