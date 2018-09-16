package migrations

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/mattes/migrate"
	redsync "gopkg.in/redsync.v1"
)

type Runner struct {
	distLock     *redsync.Mutex
	log          logutil.Log
	dbConnString string
	projectRoot  string
}

func NewRunner(distLock *redsync.Mutex, log logutil.Log, dbConnString, projectRoot string) *Runner {
	return &Runner{
		distLock:     distLock,
		log:          log,
		dbConnString: dbConnString,
		projectRoot:  projectRoot,
	}
}

func (r Runner) Run() error {
	if err := r.distLock.Lock(); err != nil {
		// distLock waits until lock will be freed
		return errors.Wrap(err, "can't acquire dist lock")
	}
	defer r.distLock.Unlock()

	migrationsDir := fmt.Sprintf("file://%s/migrations", r.projectRoot)
	m, err := migrate.New(migrationsDir, r.dbConnString)
	if err != nil {
		return fmt.Errorf("can't initialize migrations: %s", err)
	}

	if err = m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			r.log.Infof("Migrate: no ready to run migrations")
			return nil
		}

		return fmt.Errorf("can't execute migrations: %s", err)
	}

	r.log.Infof("Successfully executed database migrations")
	return nil
}
