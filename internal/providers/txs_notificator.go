package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc/simple"
	"git.zam.io/wallet-backend/web-api/pkg/services/notifications"
)

// TxsEventNotificator provides simple txs notificator
func TxsEventNotificator(transport notifications.ITransport) isc.ITxsEventNotificator {
	return simple.New(transport)
}
