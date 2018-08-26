package db

import (
	"fmt"

	cntxt "context"

	"github.com/golangci/golib/server/context"
	"github.com/jinzhu/gorm"
)

type FinishTxFunc func(*error)

type txKeyType string

var txKey txKeyType = "tx"

func getCurrentTx(ctx *context.C) *gorm.DB {
	if tx := ctx.Ctx.Value(txKey); tx != nil {
		return tx.(*gorm.DB)
	}

	return nil
}

func BeginTx(ctx *context.C) (FinishTxFunc, error) {
	if v := ctx.Ctx.Value(txKey); v != nil {
		return nil, fmt.Errorf("transaction is already started")
	}

	tx := Get(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("can't start tx: %s", tx.Error)
	}

	prevLogger := ctx.L
	ctx.L = ctx.L.WithField("tx", fmt.Sprintf("%p", tx))

	ctx.Ctx = cntxt.WithValue(ctx.Ctx, txKey, tx)

	return func(errPtr *error) {
		ctx.Ctx = cntxt.WithValue(ctx.Ctx, txKey, nil)
		*errPtr = commitOrRollbackImpl(ctx, *errPtr, tx, recover())
		ctx.L = prevLogger
	}, nil
}

func commitOrRollbackImpl(ctx *context.C, err error, tx *gorm.DB, rec interface{}) error {
	if err != nil || rec != nil {
		if e := tx.Rollback().Error; e != nil {
			// error is not nil here: we should return THAT err from the callee.
			ctx.L.Errorf("Can't rollback transaction: %s", e)
		}
		if rec != nil {
			panic(rec)
		}
	} else if e := tx.Commit().Error; e != nil {
		// we tried to commit changes but an error happened. Return 500 from callee
		// err is nil here
		err = fmt.Errorf("can't commit transaction: %s", e)
	}

	return err
}
