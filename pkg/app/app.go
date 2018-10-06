package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golangci/golangci-api/pkg/app/hooks"
	"github.com/golangci/golangci-api/pkg/cache"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/db/redis"
	"github.com/golangci/golangci-api/pkg/providers"
	apisession "github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-api/pkg/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/workers/primaryqueue/repos"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-shared/pkg/apperrors"

	"github.com/golangci/golib/server/handlers/manager"
	"github.com/gorilla/mux"

	"github.com/golangci/golangci-api/pkg/db/migrations"
	"github.com/golangci/golangci-api/pkg/services/repo"
	"github.com/golangci/golangci-api/pkg/services/repoanalysis"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/pkg/queue/aws/consumer"
	"github.com/golangci/golangci-api/pkg/queue/aws/sqs"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	_ "github.com/mattes/migrate/database/postgres" // must be first
	"gopkg.in/redsync.v1"
)

type appServices struct {
	repoanalysis repoanalysis.Service
	repo         repo.Service
}

type queues struct {
	primarySQS    *sqs.Queue
	primaryDLQSQS *sqs.Queue
}

type App struct {
	cfg              config.Config
	log              logutil.Log
	trackedLog       logutil.Log
	errTracker       apperrors.Tracker
	gormDB           *gorm.DB
	sqlDB            *sql.DB
	migrationsRunner *migrations.Runner
	services         appServices
	awsSess          *session.Session
	queues           queues
	sessFactory      *apisession.Factory
	providerFactory  *providers.Factory
	hooksInjector    *hooks.Injector
	distLockFactory  *redsync.Redsync
}

//nolint:gocyclo
func NewApp() *App {
	slog := logutil.NewStderrLog("golangci-api")
	slog.SetLevel(logutil.LogLevelInfo)

	cfg := config.NewEnvConfig(slog)

	errTracker := apperrors.GetTracker(cfg, slog, "api")
	trackedLog := apperrors.WrapLogWithTracker(slog, nil, errTracker)

	dbConnString, err := gormdb.GetDBConnString(cfg)
	if err != nil {
		slog.Fatalf("Can't get DB conn string: %s", err)
	}

	gormDB, err := gormdb.GetDB(cfg, dbConnString)
	if err != nil {
		slog.Fatalf("Can't get DB: %s", err)
	}

	sqlDB, err := gormdb.GetSQLDB(cfg, dbConnString)
	if err != nil {
		slog.Fatalf("Can't get DB: %s", err)
	}

	redisPool, err := redis.GetPool(cfg)
	if err != nil {
		slog.Fatalf("Can't get redis pool: %s", err)
	}
	rs := redsync.New([]redsync.Pool{redisPool})
	migrationsRunner := migrations.NewRunner(rs.NewMutex("migrations"), trackedLog,
		dbConnString, utils.GetProjectRoot())

	awsCfg := aws.NewConfig().WithRegion("us-east-1")
	if cfg.GetBool("AWS_DEBUG", false) {
		awsCfg = awsCfg.WithLogLevel(aws.LogDebugWithHTTPBody)
	}
	endpoint := cfg.GetString("SQS_ENDPOINT")
	if endpoint != "" {
		awsCfg = awsCfg.WithEndpoint(endpoint)
	}
	awsSess, err := session.NewSession(awsCfg)
	if err != nil {
		slog.Fatalf("Can't make aws session: %s", err)
	}

	primarySQS := sqs.NewQueue(cfg.GetString("SQS_PRIMARY_QUEUE_URL"),
		awsSess, trackedLog, primaryqueue.VisibilityTimeoutSec)
	primaryDLQSQS := sqs.NewQueue(cfg.GetString("SQS_PRIMARYDEADLETTER_QUEUE_URL"),
		awsSess, trackedLog, primaryqueue.VisibilityTimeoutSec)

	sessFactory, err := apisession.NewFactory(redisPool, cfg)
	if err != nil {
		slog.Fatalf("Failed to make session factory: %s", err)
	}

	primaryQueueProducerMultiplexer := producers.NewMultiplexer(primarySQS)
	createRepoQP := &repos.CreatorProducer{}
	if err = createRepoQP.Register(primaryQueueProducerMultiplexer); err != nil {
		slog.Fatalf("Failed to create 'create repo' producer: %s", err)
	}
	deleteRepoQP := &repos.DeleterProducer{}
	if err = deleteRepoQP.Register(primaryQueueProducerMultiplexer); err != nil {
		slog.Fatalf("Failed to create 'delete repo' producer: %s", err)
	}
	hooksInjector := &hooks.Injector{}
	providerFactory := providers.NewFactory(hooksInjector, trackedLog)
	s := appServices{
		repoanalysis: repoanalysis.BasicService{},
		repo: repo.BasicService{
			Cfg:             cfg,
			CreateQueue:     createRepoQP,
			DeleteQueue:     deleteRepoQP,
			ProviderFactory: providerFactory,
			Cache:           cache.NewRedis(cfg.GetString("REDIS_URL") + "/1"),
		},
	}

	return &App{
		cfg:              cfg,
		log:              slog,
		trackedLog:       trackedLog,
		errTracker:       errTracker,
		gormDB:           gormDB,
		sqlDB:            sqlDB,
		migrationsRunner: migrationsRunner,
		services:         s,
		awsSess:          awsSess,
		sessFactory:      sessFactory,
		hooksInjector:    hooksInjector,
		providerFactory:  providerFactory,
		distLockFactory:  rs,
		queues: queues{
			primarySQS:    primarySQS,
			primaryDLQSQS: primaryDLQSQS,
		},
	}
}

func (a App) RegisterHandlers() {
	manager.RegisterCallback(func(r *mux.Router) {
		repoanalysis.RegisterHandlers(r, a.services.repoanalysis, a.log, a.errTracker, a.gormDB, a.sessFactory)
		repo.RegisterHandlers(r, a.services.repo, a.log, a.errTracker, a.gormDB, a.sessFactory)
	})
}

func (a App) RunMigrations() {
	if err := a.migrationsRunner.Run(); err != nil {
		a.log.Fatalf("Can't run migrations: %s", err)
	}
}

func (a App) buildMultiplexedConsumer() *consumers.Multiplexer {
	primaryQueueConsumerMultiplexer := consumers.NewMultiplexer()

	repoCreatorConsumer := repos.NewCreatorConsumer(a.trackedLog, a.sqlDB, a.cfg, a.providerFactory)
	if err := repoCreatorConsumer.Register(primaryQueueConsumerMultiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo creator consumer: %s", err)
	}
	repoDeleterConsumer := repos.NewDeleterConsumer(a.trackedLog, a.sqlDB, a.cfg, a.providerFactory)
	if err := repoDeleterConsumer.Register(primaryQueueConsumerMultiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo deleter consumer: %s", err)
	}

	return primaryQueueConsumerMultiplexer
}

func (a App) RunConsumers() {
	primaryQueueConsumerMultiplexer := a.buildMultiplexedConsumer()
	primaryQueueConsumer := consumer.NewSQS(a.trackedLog, a.cfg, a.queues.primarySQS,
		primaryQueueConsumerMultiplexer, "primary", primaryqueue.VisibilityTimeoutSec)

	go primaryQueueConsumer.Run()
}

func (a App) RunDeadLetterConsumers() {
	primaryDLQConsumerMultiplexer := a.buildMultiplexedConsumer()
	primaryDLQConsumer := consumer.NewSQS(a.trackedLog, a.cfg, a.queues.primaryDLQSQS,
		primaryDLQConsumerMultiplexer, "primaryDeadLetter", primaryqueue.VisibilityTimeoutSec)

	primaryDLQConsumer.Run()
}

func (a App) RunForever() {
	a.RunMigrations()
	a.RegisterHandlers()
	a.RunConsumers()

	http.Handle("/", handlers.GetRoot())

	addr := fmt.Sprintf(":%d", a.cfg.GetInt("port", 3000))
	a.log.Infof("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		a.log.Errorf("Can't listen HTTP on %s: %s", addr, err)
		os.Exit(1)
	}
}

func (a App) GetHooksInjector() *hooks.Injector {
	return a.hooksInjector
}
