package models

import (
	"time"
)

// Coin
type Coin struct {
	ID        int64  `db:"id"`
	Name      string `db:"name"`
	ShortName string `db:"short_name"`
	Enabled   bool   `db:"enabled"`
}

// Wallet
type Wallet struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`

	UserID int64 `db:"user_id"`

	Address string `db:"address"`

	CreatedAt time.Time `db:"created_at"`

	Coin   Coin  `db:",prefix=coins_"`
	CoinID int64 `db:"coin_id"`
}
