package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
)

// ProcessingApi
func ProcessingApi(db *gorm.DB, coordinator nodes.ICoordinator, _ opentracing.Tracer) processing.IApi {
	return processing.New(db, coordinator)
}
