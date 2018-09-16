package app

import (
	"fmt"
	"net/http"
	"os"

	"github.com/golangci/golangci-api/pkg/db/redis"

	"gopkg.in/redsync.v1"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-shared/pkg/apperrors"

	"github.com/golangci/golib/server/handlers/manager"
	"github.com/gorilla/mux"

	"strings"

	"github.com/golangci/golangci-api/pkg/db/migrations"
	"github.com/golangci/golangci-api/pkg/services/repoanalysis"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	_ "github.com/mattes/migrate/database/postgres" // must be first
)

type services struct {
	repoanalysis repoanalysis.Service
}

type App struct {
	cfg              config.Config
	log              logutil.Log
	errTracker       apperrors.Tracker
	services         *services
	migrationsRunner *migrations.Runner
}

func NewApp() *App {
	slog := logutil.NewStderrLog("golangci-api")
	slog.SetLevel(logutil.LogLevelInfo)

	cfg := config.NewEnvConfig(slog)

	errTracker := apperrors.GetTracker(cfg, slog)

	dbConnString, err := getDBConnString(cfg)
	if err != nil {
		slog.Fatalf("Can't get DB conn string: %s", err)
	}

	db, err := getDB(cfg, dbConnString)
	if err != nil {
		slog.Fatalf("Can't get DB: %s", err)
	}

	s := services{
		repoanalysis: repoanalysis.BasicService{
			DB: db,
		},
	}

	redisPool, err := redis.GetPool(cfg)
	if err != nil {
		slog.Fatalf("Can't get redis pool: %s", err)
	}
	rs := redsync.New([]redsync.Pool{redisPool})
	migrationsRunner := migrations.NewRunner(rs.NewMutex("migrations"), slog,
		dbConnString, utils.GetProjectRoot())

	return &App{
		cfg:              cfg,
		log:              slog,
		errTracker:       errTracker,
		services:         &s,
		migrationsRunner: migrationsRunner,
	}
}

func (a App) RegisterHandlers() {
	manager.RegisterCallback(func(r *mux.Router) {
		repoanalysis.RegisterHandlers(r, a.services.repoanalysis, a.log, a.errTracker)
	})
}

func (a App) RunMigrations() {
	if err := a.migrationsRunner.Run(); err != nil {
		a.log.Fatalf("Can't run migrations: %s", err)
	}
}

func (a App) RunForever() {
	a.RunMigrations()
	a.RegisterHandlers()
	http.Handle("/", handlers.GetRoot())

	addr := fmt.Sprintf(":%d", a.cfg.GetInt("port", 3000))
	a.log.Infof("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		a.log.Errorf("Can't listen HTTP on %s: %s", addr, err)
		os.Exit(1)
	}
}

func getDBConnString(cfg config.Config) (string, error) {
	dbURL := cfg.GetString("DATABASE_URL")
	if dbURL == "" {
		return "", errors.New("no DATABASE_URL in config")
	}

	dbURL = strings.Replace(dbURL, "postgresql", "postgres", 1)
	return dbURL, nil
}

func getDB(cfg config.Config, connString string) (*gorm.DB, error) {
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
