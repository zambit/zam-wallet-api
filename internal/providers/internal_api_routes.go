package providers

import "github.com/gin-gonic/gin"

// InternalApiRoutes
func InternalApiRoutes(engine *gin.Engine) gin.IRouter {
	return engine.Group("/api/v1/internal")
}
