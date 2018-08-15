package server

import (
	"git.zam.io/wallet-backend/web-api/config/server"
)

// Scheme web-server params
type Scheme struct {
	// Host to listen on such address, accept both ip4 and ip6 addresses
	Host string

	// Port to listen on, negative values will cause UB
	Port int

	// JWT specific configuration, there is no default values, so if token jwt like storage is used, this must be defined
	JWT *struct {
		Secret string
		Method string
	}

	// Auth
	Auth server.AuthScheme

	// Storage
	Storage server.StorageScheme

	// Notificator
	Notificator server.NotificatorScheme
}
