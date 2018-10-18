package queries

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

	UserPhone string `db:"user_phone"`

	Address string `db:"address"`

	//TODO Move secret keys to another table
	Secret string `db:"secret"`

	CreatedAt time.Time `db:"created_at"`

	Coin   Coin  `db:",prefix=coins_" gorm:"foreignkey:CoinID;association_autoupdate:false;association_autocreate:false"`
	CoinID int64 `db:"coin_id"`
}
