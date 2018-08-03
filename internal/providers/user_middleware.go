package providers

import (
	"context"
	wmiddlewares "git.zam.io/wallet-backend/wallet-api/internal/server/middlewares"
	"git.zam.io/wallet-backend/web-api/pkg/server/handlers/base"
	"git.zam.io/wallet-backend/web-api/pkg/server/middlewares"
	"git.zam.io/wallet-backend/web-api/pkg/services/sessions"
	"github.com/gin-gonic/gin"
)

// UserMiddleware
func UserMiddleware(sessStorage sessions.IStorage) gin.HandlerFunc {
	return base.WrapMiddleware(wmiddlewares.UserMiddlewareFactory(
		func(c context.Context) (userID int64, present bool, valid bool) {
			user := middlewares.GetUserDataFromContext(c.(*gin.Context))

			rawID, present := user["id"]
			if !present {
				return
			}
			fID, valid := rawID.(float64)
			if !valid {
				return
			}
			userID = int64(fID)
			return
		},
	))
}
