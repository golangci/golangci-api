package app

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/apperrors"
	todoerrors "github.com/golangci/golangci-api/pkg/todo/errors"

	"github.com/golangci/golib/server/handlers/manager"
	"github.com/gorilla/mux"

	"strings"

	"github.com/golangci/golangci-api/pkg/config"
	"github.com/golangci/golangci-api/pkg/logutil"
	"github.com/golangci/golangci-api/pkg/services/repoanalysis"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	_ "github.com/mattes/migrate/database/postgres" // must be first
)

type services struct {
	repoanalysis repoanalysis.Service
}

type App struct {
	cfg        config.Config
	log        logutil.Log
	errTracker apperrors.Tracker
	services   *services
}

func NewApp() *App {
	slog := logutil.NewStderrLog("golangci-api")
	slog.SetLevel(logutil.LogLevelInfo)

	cfg := config.NewEnvConfig(slog)

	errTracker := todoerrors.GetTracker(cfg, slog)

	db, err := getDB(cfg)
	if err != nil {
		log.Fatalf("can't get DB: %s", err)
	}

	s := services{
		repoanalysis: repoanalysis.BasicService{
			DB: db,
		},
	}

	return &App{
		cfg:        cfg,
		log:        slog,
		errTracker: errTracker,
		services:   &s,
	}
}

func (a App) RegisterHandlers() {
	manager.RegisterCallback(func(r *mux.Router) {
		repoanalysis.RegisterHandlers(r, a.services.repoanalysis, a.log, a.errTracker)
	})
}

func (a App) RunForever() {
	a.RegisterHandlers()
	http.Handle("/", handlers.GetRoot())

	addr := fmt.Sprintf(":%d", a.cfg.GetInt("port", 3000))
	a.log.Infof("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		a.log.Errorf("Can't listen HTTP on %s: %s", addr, err)
		os.Exit(1)
	}
}

func getDB(cfg config.Config) (*gorm.DB, error) {
	dbURL := cfg.GetString("DATABASE_URL")
	if dbURL == "" {
		return nil, errors.New("no DATABASE_URL in config")
	}

	dbURL = strings.Replace(dbURL, "postgresql", "postgres", 1)
	adapter := strings.Split(dbURL, "://")[0]

	db, err := gorm.Open(adapter, dbURL)
	if err != nil {
		return nil, errors.Wrap(err, "can't open db connection")
	}

	if cfg.GetBool("DEBUG_DB", false) {
		db = db.Debug()
	}

	return db, nil
}
