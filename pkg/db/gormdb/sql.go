package gormdb

import (
	"context"
	"database/sql"

	"github.com/jinzhu/gorm"
)

type sqlDBWithContext struct {
	underlying *sql.DB
	ctx        context.Context
}

func (db *sqlDBWithContext) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.underlying.ExecContext(db.ctx, query, args...)
}

func (db *sqlDBWithContext) Prepare(query string) (*sql.Stmt, error) {
	return db.underlying.PrepareContext(db.ctx, query)
}

func (db *sqlDBWithContext) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.underlying.QueryContext(db.ctx, query, args...)
}

func (db *sqlDBWithContext) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.underlying.QueryRowContext(db.ctx, query, args...)
}

func FromSQL(ctx context.Context, db *sql.DB) (*gorm.DB, error) {
	return gorm.Open("postgres", &sqlDBWithContext{ // TODO
		underlying: db,
		ctx:        ctx,
	})
}

func FromTx(ctx context.Context, tx *sql.Tx) (*gorm.DB, error) {
	return gorm.Open("postgres", tx)
}
