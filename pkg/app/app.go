package app

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	repoanalyzeslib "github.com/golangci/golangci-api/pkg/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/db/redis"
	"github.com/golangci/golangci-api/pkg/workers/primaryqueue/repoanalyzes"

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

	"github.com/golangci/golangci-api/pkg/queue/aws/consumer"
	"github.com/golangci/golangci-api/pkg/queue/aws/sqs"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	_ "github.com/mattes/migrate/database/postgres" // must be first
)

type appServices struct {
	repoanalysis repoanalysis.Service
}

type queues struct {
	primarySQS *sqs.Queue
}

type App struct {
	cfg              config.Config
	log              logutil.Log
	trackedLog       logutil.Log
	errTracker       apperrors.Tracker
	db               *gorm.DB
	migrationsRunner *migrations.Runner
	services         appServices
	awsSess          *session.Session
	queues           queues
}

const visibilityTimeoutSec = 60 // must be in sync with cloudformation.yml

func NewApp() *App {
	slog := logutil.NewStderrLog("golangci-api")
	slog.SetLevel(logutil.LogLevelInfo)

	cfg := config.NewEnvConfig(slog)

	errTracker := apperrors.GetTracker(cfg, slog)
	trackedLog := apperrors.WrapLogWithTracker(slog, nil, errTracker)

	dbConnString, err := getDBConnString(cfg)
	if err != nil {
		slog.Fatalf("Can't get DB conn string: %s", err)
	}

	db, err := getDB(cfg, dbConnString)
	if err != nil {
		slog.Fatalf("Can't get DB: %s", err)
	}

	s := appServices{
		repoanalysis: repoanalysis.BasicService{
			DB: db,
		},
	}

	redisPool, err := redis.GetPool(cfg)
	if err != nil {
		slog.Fatalf("Can't get redis pool: %s", err)
	}
	rs := redsync.New([]redsync.Pool{redisPool})
	migrationsRunner := migrations.NewRunner(rs.NewMutex("migrations"), trackedLog,
		dbConnString, utils.GetProjectRoot())

	awsCfg := aws.NewConfig().WithRegion("us-east-1")
	endpoint := cfg.GetString("SQS_ENDPOINT")
	if endpoint != "" {
		awsCfg = awsCfg.WithEndpoint(endpoint)
	}
	awsSess, err := session.NewSession(awsCfg)
	if err != nil {
		// TODO
		trackedLog.Errorf("Can't make aws session: %s", err)
	}

	primarySQS := sqs.NewQueue(cfg.GetString("SQS_PRIMARY_QUEUE_URL"),
		awsSess, trackedLog, visibilityTimeoutSec)

	return &App{
		cfg:              cfg,
		log:              slog,
		trackedLog:       trackedLog,
		errTracker:       errTracker,
		db:               db,
		migrationsRunner: migrationsRunner,
		services:         s,
		awsSess:          awsSess,
		queues: queues{
			primarySQS: primarySQS,
		},
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

func (a App) RunConsumers() {
	if a.queues.primarySQS == nil {
		return // TODO
	}

	primaryQueueConsumerMultiplexer := consumers.NewMultiplexer()

	grsf := repoanalyzeslib.NewGithubRepoStateFetcher(a.db)
	repoanalyzesCreatorConsumer := repoanalyzes.NewCreatorConsumer(a.trackedLog, a.db, grsf)
	if err := repoanalyzesCreatorConsumer.Register(primaryQueueConsumerMultiplexer); err != nil {
		a.log.Fatalf("Failed to register repoanalyzes creator consumer: %s", err)
	}

	primaryQueueConsumer := consumer.NewSQS(a.trackedLog, a.cfg, a.queues.primarySQS,
		primaryQueueConsumerMultiplexer, "primary", visibilityTimeoutSec)

	go primaryQueueConsumer.Run()
}

func (a App) RunProducers() {
	if a.queues.primarySQS == nil {
		return // TODO
	}

	primaryQueueProducerMultiplexer := producers.NewMultiplexer(a.queues.primarySQS)

	repoanalyzesCreatorProducer := repoanalyzes.CreatorProducer{}
	if err := repoanalyzesCreatorProducer.Register(primaryQueueProducerMultiplexer); err != nil {
		a.log.Fatalf("Failed to register repoanalyzes creator producer: %s", err)
	}
}

func (a App) RunForever() {
	a.RunMigrations()
	a.RegisterHandlers()
	a.RunConsumers()
	a.RunProducers()

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
