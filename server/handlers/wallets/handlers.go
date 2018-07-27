package wallets

import (
	"git.zam.io/wallet-backend/web-api/server/handlers/base"
	"github.com/gin-gonic/gin"
)

func CreateFactory() base.HandlerFunc {
	return func(c *gin.Context) (resp interface{}, code int, err error) {
		return nil, 0, nil
	}
}
