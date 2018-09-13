package providers

import (
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/jinzhu/gorm"
	"time"
)

// Gorm
func Gorm(d *db.Db) (*gorm.DB, error) {
	gorm.NowFunc = func() time.Time {
		return time.Now().UTC()
	}

	g, err := gorm.Open("postgres", d.DB.DB)
	if err != nil {
		return nil, err
	}
	// TODO DISABLE DEBUG MODE AFTER PRODUCTION
	return g.Debug(), nil
}
