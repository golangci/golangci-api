package migrations

import (
	"fmt"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/mattes/migrate"
	_ "github.com/mattes/migrate/source/file" // must have for migrations
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

type Runner struct {
	distLock       *redsync.Mutex
	log            logutil.Log
	dbConnString   string
	migrationsPath string
}

func NewRunner(distLock *redsync.Mutex, log logutil.Log, dbConnString, migrationsPath string) *Runner {
	return &Runner{
		distLock:       distLock,
		log:            log,
		dbConnString:   dbConnString,
		migrationsPath: migrationsPath,
	}
}

func (r Runner) Run() error {
	if err := r.distLock.Lock(); err != nil {
		// distLock waits until lock will be freed
		return errors.Wrap(err, "can't acquire dist lock")
	}
	defer r.distLock.Unlock()

	migrationsDir := fmt.Sprintf("file://%s", r.migrationsPath)
	m, err := migrate.New(migrationsDir, r.dbConnString)
	if err != nil {
		return errors.Wrap(err, "can't initialize migrations")
	}

	if err = m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			r.log.Infof("Migrate: no ready to run migrations")
			return nil
		}

		return errors.Wrapf(err, "can't execute migrations in dir %s", r.migrationsPath)
	}

	r.log.Infof("Successfully executed database migrations")
	return nil
}
