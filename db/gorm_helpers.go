package db

import (
	"context"
	"github.com/jinzhu/gorm"
	ot "github.com/opentracing/opentracing-go"
)

// TransactionCtx wraps gorm transaction with instrumentation using opentracing
func TransactionCtx(ctx context.Context, db *gorm.DB, cb func(ctx context.Context, tx *gorm.DB) error) error {
	span, cCtx := ot.StartSpanFromContext(ctx, "transaction")
	defer span.Finish()

	tx := db.Begin()
	if tx.Error != nil {
		span.LogKV("open_tx_err", tx.Error)
		return tx.Error
	}
	defer func() {
		p := recover()
		if p != nil {
			span.LogKV("panic", p)
			tx.Rollback()
			panic(p)
		}
	}()

	err := cb(cCtx, tx)
	if err != nil {
		span.LogKV("cb_err", err)
		tx.Rollback()
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		span.LogKV("commit_err", err)
		return err
	}

	span.LogKV("msg", "commit_successful")
	return nil
}
