package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/web-api/db"
)

// WalletsApi
func WalletsApi(d *db.Db, coordinator nodes.ICoordinator, api processing.IApi) *wallets.Api {
	return wallets.NewApi(d, coordinator, api)
}
