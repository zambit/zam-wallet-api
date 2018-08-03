package common

import (
	"git.zam.io/wallet-backend/wallet-api/config"
	serverconf "git.zam.io/wallet-backend/wallet-api/config/server"
	walletsconf "git.zam.io/wallet-backend/wallet-api/config/wallets"
	internalproviders "git.zam.io/wallet-backend/wallet-api/internal/providers"
	"git.zam.io/wallet-backend/web-api/cmd/utils"
	dbconf "git.zam.io/wallet-backend/web-api/config/db"
	iscconf "git.zam.io/wallet-backend/web-api/config/isc"
	webserverconf "git.zam.io/wallet-backend/web-api/config/server"
	"git.zam.io/wallet-backend/web-api/pkg/providers"
	"go.uber.org/dig"
)

// ProvideBasic
func ProvideBasic(c *dig.Container, cfg config.RootScheme) {
	// provide container itself
	utils.MustProvide(c, func() *dig.Container {
		return c
	})

	// provide configuration and her parts
	utils.MustProvide(c, func() (
		config.RootScheme,
		dbconf.Scheme,
		iscconf.Scheme,
		serverconf.Scheme,
		walletsconf.Scheme,
		webserverconf.Scheme,
	) {
		servConf := cfg.Server
		wservConf := webserverconf.Scheme{
			Host:    servConf.Host,
			Port:    servConf.Port,
			Storage: servConf.Storage,
			JWT:     servConf.JWT,
			Auth:    servConf.Auth,
		}

		return cfg, cfg.DB, cfg.ISC, cfg.Server, cfg.Wallets, wservConf
	})

	// provide root logger
	utils.MustProvide(c, providers.RootLogger)

	// provide ordinal db connection
	utils.MustProvide(c, providers.DB)

	// provide nosql storage
	utils.MustProvide(c, providers.Storage)

	// provide sessions storage
	utils.MustProvide(c, providers.SessionsStorage)

	// provide broker
	utils.MustProvide(c, providers.Broker)

	// provide wallet nodes
	utils.MustProvide(c, internalproviders.Coordinator)

	// provide wallets api
	utils.MustProvide(c, internalproviders.WalletsApi)
}
