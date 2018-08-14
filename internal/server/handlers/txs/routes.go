package txs

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes         gin.IRouter     `name:"api_routes"`
	AuthMiddleware gin.HandlerFunc `name:"auth_middleware"`
	UserMiddleware gin.HandlerFunc `name:"user_middleware"`

	WalletsApi *wallets.Api
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
		base.WrapHandler(SendFactory(dependencies.WalletsApi)),
	)
	return nil
}
