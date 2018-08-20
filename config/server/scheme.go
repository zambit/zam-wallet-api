package server

import (
	"git.zam.io/wallet-backend/web-api/config/server"
)

// FrontendScheme hold configuration values which relates to front-end of this application
type FrontendScheme struct {
	// RootURL is root url of fornt-end application, used in different places by application
	RootURL string
}

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

	// InternalAccessToken token which is required to access internal endpoints
	InternalAccessToken string

	// Frontend configuration
	Frontend FrontendScheme
}
