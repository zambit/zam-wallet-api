package providers

import (
	"github.com/gin-gonic/gin"
)

// ApiRoutes
func ApiRoutes(engine *gin.Engine) gin.IRouter {
	return engine.Group("/api/v1")
}
