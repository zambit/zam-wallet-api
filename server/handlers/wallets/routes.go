package wallets

import (
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes         gin.IRouter     `name:"api_routes"`
	AuthMiddleware gin.HandlerFunc `name:"auth"`
}

// Register
func Register(container *dig.Container) {
	container.Invoke(func(dependencies Dependencies) {
		group := dependencies.Routes
		group.Use(dependencies.AuthMiddleware)
		group.POST("/wallets", base.WrapHandler(CreateFactory()))
	})
}
