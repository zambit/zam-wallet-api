package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/services/wallets"
	"git.zam.io/wallet-backend/web-api/db"
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes         gin.IRouter     `name:"api_routes"`
	AuthMiddleware gin.HandlerFunc `name:"auth_middleware"`
	UserMiddleware gin.HandlerFunc `name:"user_middleware"`
	Database       *db.Db
	Coordinator    wallets.ICoordinator
}

// Register
func Register(container *dig.Container) error {
	return container.Invoke(func(dependencies Dependencies) {
		group := dependencies.Routes.Group(
			"/user/:user_id/",
			dependencies.AuthMiddleware,
			dependencies.UserMiddleware,
		)
		group.POST("/wallets", base.WrapHandler(CreateFactory(dependencies.Database, dependencies.Coordinator)))
		group.GET("/wallets/:wallet_id", base.WrapHandler(GetFactory(dependencies.Database)))
		group.GET("/wallets", base.WrapHandler(GetAllFactory(dependencies.Database)))
	})
}
