package wallets

import (
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes         gin.IRouter     `name:"api_routes"`
	AuthMiddleware gin.HandlerFunc `name:"auth_middleware"`
	UserMiddleware gin.HandlerFunc `name:"user_middleware"`

	Api *wallets.Api

	Converter helpers.ICoinConverter
}

// Register
func Register(dependencies Dependencies) error {
	group := dependencies.Routes.Group(
		"/user/:user_phone/",
		dependencies.AuthMiddleware,
		dependencies.UserMiddleware,
	)

	group.POST(
		"/wallets",
		base.WrapHandler(CreateFactory(dependencies.Api)),
	)
	group.GET(
		"/wallets/:wallet_id",
		base.WrapHandler(GetFactory(dependencies.Api, dependencies.Converter)),
	)
	group.GET(
		"/wallets",
		base.WrapHandler(GetAllFactory(dependencies.Api, dependencies.Converter)),
	)
	return nil
}
