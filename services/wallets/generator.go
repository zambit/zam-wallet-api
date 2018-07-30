package wallets

// IGenerator used to generate user wallet for specified coin
type IGenerator interface {
	// Create coin wallet private-key - address pair
	Create(coin, seed string) (privKey, address string, err error)
}
