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

// GetWalletFilters describes wallets filters
type GetWalletFilters struct {
	Count  int64
	FromID int64
	ByCoin string
}

// GetWallets
func GetWallets(tx db.ITx, userID int64, filters GetWalletFilters) (wallets []Wallet, totalCount int64, err error) {
	// prepare where clause
	whereClause := " WHERE wallets.user_id = ?::bigint"
	limitClause := ""
	whereArgs := []interface{}{userID}
	// apply pagination
	if filters.ByCoin != "" {
		byCoin := strings.ToUpper(filters.ByCoin)
		whereClause = whereClause + " AND coins.short_name = ?"
		whereArgs = append(whereArgs, byCoin)
	}
	if filters.FromID != 0 {
		whereClause = whereClause + " AND wallets.id > ?"
		whereArgs = append(whereArgs, filters.FromID)
	}
	if filters.Count != 0 {
		limitClause = " LIMIT ?"
		whereArgs = append(whereArgs, filters.Count)
	}

	// preform query
	rows, err := tx.Query(baseSelectWalletsRequest+whereClause+limitClause, whereArgs...)
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
	err = tx.QueryRow("SELECT COUNT(*) FROM wallets "+whereClause+limitClause, whereArgs...).Scan(&totalCount)
	return
}

// GetCoin request coin by short name, returns ErrNoSuchCoin if coin doesn't exists. Coin short name argument are
// case insensitive.
func GetCoin(tx db.ITx, coinShortName string) (coin Coin, err error) {
	err = scanCoinRow(
		tx.QueryRow(
			`SELECT id, name, short_name, enabled FROM coins WHERE short_name = $1`,
			strings.ToUpper(coinShortName),
		),
		&coin,
	)
	if err == sql.ErrNoRows {
		err = ErrNoSuchCoin
	}
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

func scanCoinRow(row scannable, coin *Coin) error {
	return row.Scan(
		&coin.ID,
		&coin.Name,
		&coin.ShortName,
		&coin.Enabled,
	)
}
