package nodes

// IGenerator used to generate user wallet for specified coin
type IGenerator interface {
	// Create coin wallet address
	Create() (address string, err error)
}
