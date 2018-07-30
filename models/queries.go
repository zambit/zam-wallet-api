package models

import (
	"database/sql"
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"strings"
)

var (
	// ErrNoSuchCoin returned when invalid coin name is specified
	ErrNoSuchCoin = errors.New("such coin name is unexpected")

	// ErrNoSuchWallet returned when no wallet found for specified criteria
	ErrNoSuchWallet = errors.New("no such wallet found")
)

// CreateWallet
func CreateWallet(tx db.ITx, wallet Wallet) (newWallet Wallet, err error) {
	err = tx.QueryRow(
		`INSERT INTO wallets (name, user_id, address, coin_id)
         VALUES ($1, $2, $3, (SELECT id FROM coins WHERE short_name = $4 AND enabled = true))
         RETURNING id`,
		wallet.Name, wallet.UserID, wallet.Address, strings.ToUpper(wallet.Coin.ShortName),
	).Scan(&wallet.ID)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Column == "coin_id" || pgErr.Code == "23502" {
				err = ErrNoSuchCoin
			}
		}
		return
	}
	newWallet = wallet
	return
}

const baseSelectWalletsRequest = `
SELECT
	wallets.id,
	wallets.user_id,
	wallets.coin_id, 
	wallets.name, 
	wallets.address,
	wallets.created_at,
	coins.name,
	coins.short_name
FROM wallets
INNER JOIN coins ON coins.id = wallets.coin_id`

// GetWallet
func GetWallet(tx db.ITx, userID int64, walletID int64) (wallet Wallet, err error) {
	err = scanWalletRow(tx.QueryRow(
		baseSelectWalletsRequest+` WHERE wallets.id = $1 AND wallets.user_id = $2::bigint`, walletID, userID,
	), &wallet)
	if err == sql.ErrNoRows {
		err = ErrNoSuchWallet
	}
	return
}

// GetWallets
func GetWallets(tx db.ITx, userID int64) (wallets []Wallet, totalCount int64, err error) {
	rows, err := tx.Query(baseSelectWalletsRequest+` WHERE wallets.user_id = $1::bigint`, userID)
	if err != nil {
		return
	}

	// query wallets
	wallets = make([]Wallet, 0, 3)
	for rows.Next() {
		var wallet Wallet
		err = scanWalletRow(rows, &wallet)
		if err != nil {
			return
		}
		wallets = append(wallets, wallet)
	}

	// query total count
	err = tx.QueryRow("SELECT COUNT(*) FROM wallets WHERE user_id = $1::bigint", userID).Scan(&totalCount)
	return
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanWalletRow(row scannable, wallet *Wallet) error {
	return row.Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.CoinID,
		&wallet.Name,
		&wallet.Address,
		&wallet.CreatedAt,
		&wallet.Coin.Name,
		&wallet.Coin.ShortName,
	)
}
