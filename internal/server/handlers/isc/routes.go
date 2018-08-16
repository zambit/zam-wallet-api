package isc

import (
	"git.zam.io/wallet-backend/wallet-api/config/server"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets"
	"git.zam.io/wallet-backend/wallet-api/pkg/services/convert"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"net/http"
	"strings"
)

// Dependencies
type Dependencies struct {
	dig.In

	Routes     gin.IRouter `name:"internal_api_routes"`
	Config     server.Scheme
	WalletsApi *wallets.Api
	Converter  convert.ICryptoCurrency
}

// Register
func Register(dependencies Dependencies) error {
	dependencies.Routes.GET(
		"/user_stat",
		trace.StartSpanMiddleware(),
		base.WrapMiddleware(TokenAuthMiddlewareFactory(dependencies.Config.InternalAccessToken)),
		base.WrapHandler(UserStatFactory(dependencies.WalletsApi, dependencies.Converter)),
	)
	return nil
}

// TokenAuthMiddlewareFactory
func TokenAuthMiddlewareFactory(expectsToken string) base.HandlerFunc {
	const bearerPrefix = "Bearer"
	errUnauthorized := base.ErrorView{Code: http.StatusUnauthorized, Message: "unauthorized"}

	return func(c *gin.Context) (resp interface{}, code int, err error) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			err = errUnauthorized
			return
		}

		// validate header
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 {
			err = errUnauthorized
			return
		}
		if parts[0] != bearerPrefix {
			err = errUnauthorized
			return
		}
		if parts[1] != expectsToken {
			err = errUnauthorized
			return
		}

		return
	}
}