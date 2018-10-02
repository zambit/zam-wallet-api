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
	//
	// Default values is 6
	NeedConfirmationsCount int
}

// ETHNodeConfiguration defines additional configuration params required for ETH-like nodes
type ETHNodeConfiguration struct {
	// NeedConfirmationsCount specifies number of confirmations which is required to treat a tx as confirmed
	//
	// Default values is 12
	NeedConfirmationsCount int

	// EtherscanHost and EtherscanApiKey specifies etherscan params, there is no default values!
	EtherscanHost, EtherscanApiKey string

	// MasterPass used to unlock wallets
	MasterPass string
}

type ZAMNodeConfiguration struct {
	AssetName       string
	IssuerPublicKey string
}

type Scheme struct {
	// CryptoNodes contains per crypto-coin connections info
	CryptoNodes map[string]NodeConnection

	// UserReporter user reporter
	UserReporter bool

	// BTC holds additional BTC-like node configuration values
	BTC BTCNodeConfiguration

	// ETH holds additional ETH-like node configuration values
	ETH ETHNodeConfiguration

	// ETH holds additional ETH-like node configuration values
	ZAM ZAMNodeConfiguration
}
