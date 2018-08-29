package wallets

// NodeConnection describes node connection params
type NodeConnection struct {
	// Host may contains port in format "host:port", in such case Testnet arg will be ignored
	Host string
	// User and Pass for basic auth
	User, Pass string
	// Testnet ignored if port passed inside Host param
	Testnet bool
}

// BTCNodeConfiguration defines additional configuration values required for BTC-like nodes
type BTCNodeConfiguration struct {
	// NeedConfirmationsCount specifies number of confirmations which is required to treat a tx as confirmed
	NeedConfirmationsCount int
}

type Scheme struct {
	// CryptoNodes contains per crypto-coin connections info
	CryptoNodes map[string]NodeConnection

	// BTC holds additional BTC-like node configuration values
	BTC BTCNodeConfiguration
}
