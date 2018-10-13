package gormdb

import (
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type FinishTxFunc func(err *error)

func StartTx(db *gorm.DB) (*gorm.DB, FinishTxFunc, error) {
	tx := db.Begin()
	if tx.Error != nil {
		return nil, nil, errors.Wrap(tx.Error, "failed to start transaction")
	}

	return tx, func(err *error) {
		finishTx(tx, err, recover())
	}, nil
}

func finishTx(tx *gorm.DB, err *error, rec interface{}) {
	if rec != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			*err = errors.Wrapf(rollbackErr, "failed to rollback transaction after panic: %s", rec)
		}
		return
	}

	if *err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			*err = errors.Wrapf(*err, "failed to rollback transaction: %s", rollbackErr)
		}
		return
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		*err = errors.Wrap(commitErr, "failed to commit transaction")
	}
}
