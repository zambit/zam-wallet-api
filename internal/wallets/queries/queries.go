package queries

import (
	"database/sql"
	"fmt"
	"git.zam.io/wallet-backend/wallet-api/internal/wallets/errs"
	"git.zam.io/wallet-backend/web-api/db"
	"github.com/lib/pq"
	"strings"
)

const (
	notNullViolationErrCode          = "23502"
	uniqueConstraintViolationErrCode = "23505"
)

// CreateWallet creates wallet using wallet internal coin short name to lookup appropriate coin id, if there is no such
// coin with given short name (note: coin short name is case insensitive), ErrNoSuchCoin will be returned.
//
// Also in attempt to create wallet which broke unique user_id and coin_id constraint, ErrWalletCreationRejected
// will be returned.
func CreateWallet(tx db.ITx, wallet Wallet) (newWallet Wallet, err error) {
	err = tx.QueryRowx(
		`INSERT INTO wallets (name, user_id, address, coin_id)
         VALUES ($1, $2, $3, (SELECT id FROM coins WHERE short_name = $4 AND enabled = true))
         RETURNING id`,
		wallet.Name, wallet.UserID, wallet.Address, strings.ToUpper(wallet.Coin.ShortName),
	).Scan(&wallet.ID)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			switch {
			case pgErr.Column == "coin_id" || pgErr.Code == notNullViolationErrCode:
				err = errs.ErrNoSuchCoin
			case pgErr.Code == uniqueConstraintViolationErrCode:
				err = errs.ErrWalletCreationRejected
			}
		}
		if err == sql.ErrNoRows {
			panic(fmt.Errorf("create wallet: named query row retuns no rows"))
		}
		return
	}
	newWallet = wallet
	return
}

// WalletDiff used by update request
type WalletDiff struct {
	Name, Address *string
	CoinID        *int64
}

const (
	baseUpdateWalletRequest     = `UPDATE wallets SET `
	appendixUpdateWalletRequest = ` WHERE wallets.id = :wallet_id RETURNING wallets.id`
)

// UpdateWallet updates wallet db row which matched by passed id using non-nil field from diff argument.
//
// Returns ErrNoSuchWallet if wallet id is invalid
func UpdateWallet(tx db.ITx, id int64, diff *WalletDiff) (err error) {
	colNames := make([]string, 0, 4)
	colArgs := make([]interface{}, 0, 4)

	// check diffs
	if diff.Name != nil {
		colNames = append(colNames, "name")
		colArgs = append(colArgs, *diff.Name)
	}

	if diff.Address != nil {
		colNames = append(colNames, "address")
		colArgs = append(colArgs, *diff.Address)
	}

	if diff.CoinID != nil {
		colNames = append(colNames, "coin_id")
		colArgs = append(colArgs, *diff.CoinID)
	}

	// we don't really want query empty update statement
	if len(colNames) == 0 {
		return nil
	}

	// construct sql using builder to speed up variate appends
	builder := strings.Builder{}
	builder.WriteString(baseUpdateWalletRequest)

	namedArgs := make(map[string]interface{}, len(colNames)+1)
	namedArgs["wallet_id"] = id

	updateRows := make([]string, 0, len(colNames))
	for i, arg := range colArgs {
		name := colNames[i]

		namedArgs[name] = arg
		updateRows = append(updateRows, name+" = :"+name)
	}

	builder.WriteString(strings.Join(updateRows, ", "))
	builder.WriteString(appendixUpdateWalletRequest)

	// perform actual query
	// need to call scan in order to ensure that request has been performed successfully
	err = tx.NamedQueryRow(builder.String(), namedArgs).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			panic(fmt.Errorf("update wallet: named query row retuns no rows"))
		}
	}
	return
}

const (
	baseSelectWalletsRequest = `
SELECT
	wallets.id,
	wallets.user_id,
	wallets.coin_id, 
	wallets.name, 
	wallets.address,
	wallets.created_at,
	coins.id as coins_id,
    coins.name as coins_name,
    coins.short_name as coins_short_name,
    coins.enabled as coins_enabled
FROM wallets
INNER JOIN coins ON coins.id = wallets.coin_id`

	appendixSelectWalletsRequest = " ORDER BY wallets.id ASC"
)

// GetWallet
func GetWallet(tx db.ITx, userID int64, walletID int64, forUpdate ...bool) (wallet Wallet, err error) {
	forUpdateClause := ""
	if len(forUpdate) > 0 && forUpdate[0] {
		forUpdateClause = " FOR UPDATE"
	}

	err = scanWalletRow(tx.QueryRowx(
		baseSelectWalletsRequest+` WHERE wallets.id = $1 AND wallets.user_id = $2::bigint `+forUpdateClause,
		walletID, userID,
	), &wallet)
	if err == sql.ErrNoRows {
		err = errs.ErrNoSuchWallet
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
func GetWallets(tx db.ITx, userID int64, filters GetWalletFilters) (
	wallets []Wallet, totalCount int64, hasNext bool, err error,
) {
	// prepare where clause
	whereClause := " WHERE wallets.user_id = :user_id ::::bigint"
	limitClause := ""
	whereArgs := map[string]interface{}{"user_id": userID}

	if filters.ByCoin != "" {
		// apply coin filter
		byCoin := strings.ToUpper(filters.ByCoin)
		whereClause = whereClause + " AND coins.short_name = :coin_short_name"
		whereArgs["coin_short_name"] = byCoin
	}
	if filters.FromID != 0 {
		// apply pagination
		whereClause = whereClause + " AND wallets.id > :wallet_id"
		whereArgs["wallet_id"] = filters.FromID
	}
	if filters.Count != 0 {
		// apply limit
		limitClause = " LIMIT :limit"
		whereArgs["limit"] = filters.Count
	}

	// preform query
	rows, err := tx.NamedQuery(baseSelectWalletsRequest+whereClause+appendixSelectWalletsRequest+limitClause, whereArgs)
	if err != nil {
		return
	}
	defer rows.Close()

	// query wallets
	wallets = make([]Wallet, 0, 3)

	// need to scan all wallets
	for rows.Next() {
		var wallet Wallet
		err = scanWalletRow(rows, &wallet)
		if err != nil {
			return
		}
		wallets = append(wallets, wallet)
	}

	// query total count
	var lastIDWithCountRes struct {
		Count  int64  `db:"count"`
		LastID *int64 `db:"last_id"`
	}
	// detect has next flag by querying last wallet id with same where clause, if last id not equal to id of last select
	// wallet that mean that there more to select
	err = tx.NamedQueryRow(`WITH last_id_select AS (
            SELECT wallets.id FROM wallets INNER JOIN coins ON coins.id = wallets.coin_id`+whereClause+` ORDER BY id DESC LIMIT 1
		)
		SELECT COUNT(*) AS count, (SELECT id FROM last_id_select) AS last_id FROM wallets
		INNER JOIN coins ON coins.id = wallets.coin_id`+whereClause+limitClause,
		whereArgs,
	).StructScan(&lastIDWithCountRes)
	if err != nil {
		return
	}

	totalCount = lastIDWithCountRes.Count
	hasNext = lastIDWithCountRes.LastID != nil && *lastIDWithCountRes.LastID != wallets[len(wallets)-1].ID
	return
}

// GetCoin request coin by short name, returns ErrNoSuchCoin if coin doesn't exists. Coin short name argument are
// case insensitive.
func GetCoin(tx db.ITx, coinShortName string) (coin Coin, err error) {
	err = tx.QueryRowx(
		`SELECT id, name, short_name, enabled FROM coins WHERE short_name = $1 AND enabled = true`,
		strings.ToUpper(coinShortName),
	).StructScan(&coin)
	if err == sql.ErrNoRows {
		err = errs.ErrNoSuchCoin
	}
	return
}

// GetCoins gets all coins which created to the used by default
func GetDefaultCoins(tx db.ITx) (coins []Coin, err error) {
	q := `SELECT id, name, short_name, enabled FROM coins WHERE user_default = true AND enabled = true`

	rows, err := tx.Queryx(q)
	if err != nil {
		return
	}
	defer rows.Close()

	// query coins
	coins = make([]Coin, 0, 3)

	// need to scan all wallets
	for rows.Next() {
		var coin Coin
		err = rows.StructScan(&coin)
		if err != nil {
			return
		}
		coins = append(coins, coin)
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
		&wallet.Coin.ID,
		&wallet.Coin.Name,
		&wallet.Coin.ShortName,
		&wallet.Coin.Enabled,
	)
}
