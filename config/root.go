package config

import (
	"git.zam.io/wallet-backend/wallet-api/config/server"
	"git.zam.io/wallet-backend/wallet-api/config/wallets"
	"git.zam.io/wallet-backend/web-api/config/db"
	"github.com/spf13/viper"
)

// RootScheme is the scheme used by top-level app
type RootScheme struct {
	// Env describes current environment
	Env string

	// DB connection description
	DB db.Scheme

	// Server holds different web-server related configuration values
	Server server.Scheme

	// Wallets configuration
	Wallets wallets.Scheme
}

// Init set default values
func Init(v *viper.Viper) {
	v.SetDefault("Env", "test")
	v.SetDefault("Db.Uri", "postgresql://postgres:postgres@localhost:5432/postgres")
	v.SetDefault("Server.Auth.TokenName", "Bearer")
	v.SetDefault("Server.Host", "localhost")
	v.SetDefault("Server.Port", 9998)
	v.SetDefault("Server.Storage.URI", "mem://")
}
