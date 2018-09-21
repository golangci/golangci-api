package gormdb

import (
	"database/sql"

	"github.com/pkg/errors"
)

func FinishTx(tx *sql.Tx, err *error) {
	if *err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			*err = errors.Wrapf(*err, "Failed to rollback transaction: %s", rollbackErr)
		}
		return
	}

	if commitErr := tx.Commit(); commitErr != nil {
		*err = errors.Wrap(commitErr, "failed to commit transaction")
	}
}
