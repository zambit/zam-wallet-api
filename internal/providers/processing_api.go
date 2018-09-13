package providers

import (
	processingconf "git.zam.io/wallet-backend/wallet-api/config/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers"
	"git.zam.io/wallet-backend/wallet-api/internal/helpers/balance"
	"git.zam.io/wallet-backend/wallet-api/internal/processing"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/nodes"
	"github.com/jinzhu/gorm"
	"github.com/opentracing/opentracing-go"
)

// ProcessingApi
func ProcessingApi(
	db *gorm.DB,
	coordinator nodes.ICoordinator,
	_ opentracing.Tracer,
	txNotificator isc.ITxsEventNotificator,
) (processing.IApi, helpers.IBalance) {
	b := balance.New(coordinator, nil)
	api := processing.New(db, b, txNotificator, coordinator)
	b.ProcessingApi = api
	return api, b
}

// ConfirmationsNotifier
func ConfirmationsNotifier(db *gorm.DB, coordinator nodes.ICoordinator) processing.IConfirmationNotifier {
	return processing.NewConfirmationsNotifier(db, coordinator)
}

// CheckOutdatedNotifier
func CheckOutdatedNotifier(db *gorm.DB, cfg processingconf.Scheme) processing.ICheckOutdatedNotifier {
	return processing.NewCheckOutdatedNotifier(db, cfg.TimeToWaitRecipient)
}
