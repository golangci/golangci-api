package shared

import (
	"fmt"
	"os"

	_ "github.com/mattes/migrate/database/postgres" // must be first

	"github.com/golangci/golangci-api/app/internal/analyzes"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-worker/app/utils/queue"
	"github.com/golangci/golib/server/database"

	log "github.com/sirupsen/logrus"

	"github.com/mattes/migrate"
	_ "github.com/mattes/migrate/source/file" // must have for migrations
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)
}

func Init() {
	if err := runMigrations(); err != nil {
		log.Fatalf("Can't run db migrations: %s", err)
	}

	queue.Init()

	analyzes.StartWatcher()
}

func runMigrations() error {
	dbConfig, err := database.GetDBConfig()
	if err != nil {
		return fmt.Errorf("can't get db config: %s", err)
	}

	// TODO: acquire lock in redis here to support > 1 instances
	migrationsDir := fmt.Sprintf("file://%s/app/migrations", utils.GetProjectRoot())
	m, err := migrate.New(migrationsDir, dbConfig.ConnString)
	if err != nil {
		return fmt.Errorf("can't initialize migrations: %s", err)
	}

	if err = m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Infof("Migrate: no ready to run migrations")
			return nil
		}

		return fmt.Errorf("can't execute migrations: %s", err)
	}

	log.Infof("Successfully executed database migrations")
	return nil
}
