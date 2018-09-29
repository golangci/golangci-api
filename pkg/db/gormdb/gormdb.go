package gormdb

import (
	"database/sql"
	"strings"

	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func GetDBConnString(cfg config.Config) (string, error) {
	dbURL := cfg.GetString("DATABASE_URL")
	if dbURL == "" {
		return "", errors.New("no DATABASE_URL in config")
	}

	dbURL = strings.Replace(dbURL, "postgresql", "postgres", 1)
	return dbURL, nil
}

func GetDB(cfg config.Config, connString string) (*gorm.DB, error) {
	adapter := strings.Split(connString, "://")[0]

	db, err := gorm.Open(adapter, connString)
	if err != nil {
		return nil, errors.Wrap(err, "can't open db connection")
	}

	if cfg.GetBool("DEBUG_DB", false) {
		db = db.Debug()
	}

	return db, nil
}

func GetSQLDB(cfg config.Config, connString string) (*sql.DB, error) {
	adapter := strings.Split(connString, "://")[0]

	db, err := sql.Open(adapter, connString)
	if err != nil {
		return nil, errors.Wrap(err, "can't open db connection")
	}

	return db, nil
}
