package models

import (
	"time"
)

// Coin
type Coin struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

// Wallet
type Wallet struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`

	UserID int64 `db:"user_id"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`

	Coin   CoinType
	CoinID int64 `db:"coin_id"`
}
