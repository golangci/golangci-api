package shared

import (
	"os"

	_ "github.com/mattes/migrate/database/postgres" // must be first

	"github.com/golangci/golangci-api/pkg/analyzes"
	"github.com/golangci/golangci-api/pkg/analyzes/repoanalyzes"
	"github.com/golangci/golangci-worker/app/lib/queue"

	log "github.com/sirupsen/logrus"

	_ "github.com/mattes/migrate/source/file" // must have for migrations

	_ "github.com/golangci/golangci-api/app/handlers/auth"   // register handler
	_ "github.com/golangci/golangci-api/app/handlers/events" // register handler
	_ "github.com/golangci/golangci-api/app/handlers/repos"  // register handler
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.InfoLevel)
}

func Init() {
	queue.Init()

	analyzes.StartWatcher()
	repoanalyzes.Start()
}
