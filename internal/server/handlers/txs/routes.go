package txs

import (
	"git.zam.io/wallet-backend/wallet-api/internal/txs"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes         gin.IRouter     `name:"api_routes"`
	AuthMiddleware gin.HandlerFunc `name:"auth_middleware"`
	UserMiddleware gin.HandlerFunc `name:"user_middleware"`

	WalletsApi *wallets.Api
	TxsApi     txs.IApi
	Converter  convert.ICryptoCurrency
}

// Register
func Register(dependencies Dependencies) error {
	group := dependencies.Routes.Group(
		"/user/:user_phone/",
		trace.StartSpanMiddleware(),
		dependencies.AuthMiddleware,
		dependencies.UserMiddleware,
	)

	group.POST(
		"/txs",
		base.WrapHandler(SendFactory(dependencies.WalletsApi, dependencies.Converter)),
	)
	group.GET(
		"/txs/:tx_id",
		base.WrapHandler(GetFactory(dependencies.TxsApi, dependencies.Converter)),
	)
	group.GET(
		"/txs",
		base.WrapHandler(GetAllFactory(dependencies.TxsApi, dependencies.Converter)),
	)
	return nil
}
