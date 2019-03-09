package gormdb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/jinzhu/gorm"
	_ "github.com/mattes/migrate/database/postgres" // init pg driver
	"github.com/pkg/errors"
)

func GetDBConnString(cfg config.Config) (string, error) {
	dbURL := cfg.GetString("DATABASE_URL")
	if dbURL != "" {
		dbURL = strings.Replace(dbURL, "postgresql", "postgres", 1)
		return dbURL, nil
	}

	host := cfg.GetString("DATABASE_HOST")
	username := cfg.GetString("DATABASE_USERNAME")
	password := cfg.GetString("DATABASE_PASSWORD")
	name := cfg.GetString("DATABASE_NAME")
	if host == "" || username == "" || password == "" || name == "" {
		return "", errors.New("no DATABASE_URL or DATABASE_{HOST,USERNAME,PASSWORD,NAME} in config")
	}

	//TODO: enable SSL, but it's not critical
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", username, password, host, name), nil
}

func GetDB(cfg config.Config, log logutil.Log, connString string) (*gorm.DB, error) {
	if connString == "" {
		var err error
		connString, err = GetDBConnString(cfg)
		if err != nil {
			return nil, err
		}
	}
	adapter := strings.Split(connString, "://")[0]
	isDebug := cfg.GetBool("DEBUG_DB", false)
	if isDebug {
		log.Infof("Connecting to database %s", connString)
	}

	db, err := gorm.Open(adapter, connString)
	if err != nil {
		return nil, errors.Wrap(err, "can't open db connection")
	}

	if isDebug {
		db = db.Debug()
	}

	db.SetLogger(logger{
		log: log,
	})

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
