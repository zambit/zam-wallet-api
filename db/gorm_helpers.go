package db

import (
	"context"
	"github.com/jinzhu/gorm"
	ot "github.com/opentracing/opentracing-go"
	"git.zam.io/wallet-backend/wallet-api/pkg/trace"
)

// TransactionCtx wraps gorm transaction with instrumentation using opentracing
func TransactionCtx(ctx context.Context, db *gorm.DB, cb func(ctx context.Context, tx *gorm.DB) error) error {
	span, cCtx := ot.StartSpanFromContext(ctx, "transaction")
	defer span.Finish()

	tx := db.Begin()
	if tx.Error != nil {
		trace.LogErrorWithMsg(span, tx.Error, "error occurs while opening transaction")
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
		trace.LogErrorWithMsg(span, err, "error returned from callback")
		tx.Rollback()
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		trace.LogErrorWithMsg(span, err, "error occurs while committing transaction")
		return err
	}

	trace.LogMsg(span, "transaction committed successfully")
	return nil
}
