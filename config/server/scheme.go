package server

import (
	"git.zam.io/wallet-backend/web-api/config/server"
	"time"
)

// FrontendScheme hold configuration values which relates to front-end of this application
type FrontendScheme struct {
	// RootURL is root url of fornt-end application, used in different places by application
	RootURL string
}

// ConvertScheme holds configuration settings used to convert
type ConvertScheme struct {
	// Type main converter type, either is 'icex' or 'cryptocompare' value.
	Type string

	// FallbackType same as Type, but specifies type of another converter service, which will be used if answer can't
	// be obtained in Timeout period from the main converter. Set same values as Type is't recommended, but accepted.
	FallbackType string

	// FallbackTimeout specifies time for which answer from main host will be awaited before fallback.
	FallbackTimeout time.Duration
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

	// Convert in-app converter configuration
	Convert ConvertScheme

	// Frontend configuration
	Frontend FrontendScheme
}
