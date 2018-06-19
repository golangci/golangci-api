package db

import (
	"fmt"
	"os"

	"github.com/golangci/golib/server/handlers/herrors"

	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/database"
	"github.com/jinzhu/gorm"
)

func Get(ctx *context.C) *gorm.DB {
	if tx := getCurrentTx(ctx); tx != nil {
		return tx
	}

	DB := database.GetDB()
	isDebug := os.Getenv("DATABASE_DEBUG") == "1"
	if isDebug {
		DB = DB.Debug()
	}
	DB.SetLogger(logger{
		ctx: ctx,
	})

	return DB
}

func Error(err error, format string, args ...interface{}) error {
	if err == gorm.ErrRecordNotFound {
		errBegin := fmt.Sprintf(format, args...)
		return herrors.New404Errorf("%s: %s", errBegin, err)
	}

	return herrors.New(err, format, args...)
}
