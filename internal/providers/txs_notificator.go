package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc"
	"git.zam.io/wallet-backend/wallet-api/internal/services/isc/simple"
	"git.zam.io/wallet-backend/web-api/pkg/services/notifications"
	"git.zam.io/wallet-backend/wallet-api/config/server"
)

// TxsEventNotificator provides simple txs notificator
func TxsEventNotificator(server server.Scheme, transport notifications.ITransport) isc.ITxsEventNotificator {
	return simple.New(transport, server.Frontend.RootURL)
}
