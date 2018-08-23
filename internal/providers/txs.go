package providers

import (
	"git.zam.io/wallet-backend/wallet-api/internal/txs"
	"github.com/jinzhu/gorm"
)

// TxsApi provides default txs api implementation
func TxsApi(db *gorm.DB) txs.IApi {
	return txs.New(db)
}
