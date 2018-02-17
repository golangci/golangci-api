package db

import (
	"os"

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
