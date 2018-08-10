package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers/balance"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
)

// ProcessingApi
func ProcessingApi(db *gorm.DB, coordinator nodes.ICoordinator, _ opentracing.Tracer) (processing.IApi, helpers.IBalance) {
	b := balance.New(coordinator, nil)
	api := processing.New(db, coordinator, b)
	b.ProcessingApi = api
	return api, b
}
