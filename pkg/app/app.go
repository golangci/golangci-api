package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/golangci/golangci-api/pkg/services/pranalysis"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	redigo "github.com/garyburd/redigo/redis"
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
	"github.com/golangci/golangci-api/pkg/transportutil"
	"github.com/golangci/golangci-shared/pkg/apperrors"

	"github.com/golangci/golib/server/handlers/manager"
	"github.com/gorilla/mux"

	"github.com/golangci/golangci-api/pkg/db/migrations"
	"github.com/golangci/golangci-api/pkg/services/repo"
	"github.com/golangci/golangci-api/pkg/services/repoanalysis"
	"github.com/golangci/golangci-api/pkg/services/repohook"
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
	repohook     repohook.Service
	pranalysis   pranalysis.Service
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
	providerFactory  providers.Factory
	hooksInjector    *hooks.Injector
	distLockFactory  *redsync.Redsync
	redisPool        *redigo.Pool
}

//nolint:gocyclo
func (a *App) buildDeps() {
	if a.log == nil {
		slog := logutil.NewStderrLog("golangci-api")
		slog.SetLevel(logutil.LogLevelInfo)
		a.log = slog
	}

	if a.cfg == nil {
		a.cfg = config.NewEnvConfig(a.log)
	}

	if a.errTracker == nil {
		a.errTracker = apperrors.GetTracker(a.cfg, a.log, "api")
	}
	if a.trackedLog == nil {
		a.trackedLog = apperrors.WrapLogWithTracker(a.log, nil, a.errTracker)
	}

	if a.gormDB == nil || a.sqlDB == nil {
		dbConnString, err := gormdb.GetDBConnString(a.cfg)
		if err != nil {
			a.log.Fatalf("Can't get DB conn string: %s", err)
		}

		if a.gormDB == nil {
			gormDB, err := gormdb.GetDB(a.cfg, dbConnString)
			if err != nil {
				a.log.Fatalf("Can't get DB: %s", err)
			}
			a.gormDB = gormDB
		}

		if a.sqlDB == nil {
			sqlDB, err := gormdb.GetSQLDB(a.cfg, dbConnString)
			if err != nil {
				a.log.Fatalf("Can't get DB: %s", err)
			}
			a.sqlDB = sqlDB
		}
	}

	if a.hooksInjector == nil {
		a.hooksInjector = &hooks.Injector{}
	}
	if a.providerFactory == nil {
		a.providerFactory = providers.NewBasicFactory(a.hooksInjector, a.trackedLog)
	}

	if a.redisPool == nil {
		redisPool, err := redis.GetPool(a.cfg)
		if err != nil {
			a.log.Fatalf("Can't get redis pool: %s", err)
		}
		a.redisPool = redisPool
	}
}

func (a *App) buildAwsSess() {
	awsCfg := aws.NewConfig().WithRegion("us-east-1")
	if a.cfg.GetBool("AWS_DEBUG", false) {
		awsCfg = awsCfg.WithLogLevel(aws.LogDebugWithHTTPBody)
	}
	endpoint := a.cfg.GetString("SQS_ENDPOINT")
	if endpoint != "" {
		awsCfg = awsCfg.WithEndpoint(endpoint)
	}
	awsSess, err := session.NewSession(awsCfg)
	if err != nil {
		a.log.Fatalf("Can't make aws session: %s", err)
	}
	a.awsSess = awsSess
}

func (a *App) buildQueues() {
	a.queues.primarySQS = sqs.NewQueue(a.cfg.GetString("SQS_PRIMARY_QUEUE_URL"),
		a.awsSess, a.trackedLog, primaryqueue.VisibilityTimeoutSec)
	a.queues.primaryDLQSQS = sqs.NewQueue(a.cfg.GetString("SQS_PRIMARYDEADLETTER_QUEUE_URL"),
		a.awsSess, a.trackedLog, primaryqueue.VisibilityTimeoutSec)
}

func (a *App) buildServices() {
	a.services.repoanalysis = repoanalysis.BasicService{}
	a.services.repohook = repohook.BasicService{
		ProviderFactory: a.providerFactory,
	}
	a.services.pranalysis = pranalysis.BasicService{}

	a.buildRepoService()
}

func (a *App) buildRepoService() {
	primaryQueueProducerMultiplexer := producers.NewMultiplexer(a.queues.primarySQS)
	createRepoQP := &repos.CreatorProducer{}
	if err := createRepoQP.Register(primaryQueueProducerMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'create repo' producer: %s", err)
	}
	deleteRepoQP := &repos.DeleterProducer{}
	if err := deleteRepoQP.Register(primaryQueueProducerMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'delete repo' producer: %s", err)
	}

	a.services.repo = repo.BasicService{
		Cfg:             a.cfg,
		CreateQueue:     createRepoQP,
		DeleteQueue:     deleteRepoQP,
		ProviderFactory: a.providerFactory,
		Cache:           cache.NewRedis(a.cfg.GetString("REDIS_URL") + "/1"),
	}
}

func (a *App) buildSessFactory() {
	sessFactory, err := apisession.NewFactory(a.redisPool, a.cfg)
	if err != nil {
		a.log.Fatalf("Failed to make session factory: %s", err)
	}
	a.sessFactory = sessFactory
}

func (a *App) buildMigrationsRunner() {
	a.distLockFactory = redsync.New([]redsync.Pool{a.redisPool})
	dbConnString, err := gormdb.GetDBConnString(a.cfg)
	if err != nil {
		a.log.Fatalf("Can't get DB conn string: %s", err)
	}
	a.migrationsRunner = migrations.NewRunner(a.distLockFactory.NewMutex("migrations"), a.trackedLog,
		dbConnString, utils.GetProjectRoot())
}

func NewApp(modifiers ...Modifier) *App {
	a := App{}
	for _, m := range modifiers {
		m(&a)
	}
	a.buildDeps()
	a.buildAwsSess()
	a.buildQueues()
	a.buildSessFactory()
	a.buildServices()
	a.buildMigrationsRunner()

	return &a
}

func (a App) RegisterHandlers() {
	manager.RegisterCallback(func(r *mux.Router) {
		regCtx := &transportutil.HandlerRegContext{
			Router:      r,
			Log:         a.log,
			ErrTracker:  a.errTracker,
			DB:          a.gormDB,
			SessFactory: a.sessFactory,
		}
		repoanalysis.RegisterHandlers(a.services.repoanalysis, regCtx)
		repo.RegisterHandlers(a.services.repo, regCtx)
		repohook.RegisterHandlers(a.services.repohook, regCtx)
		pranalysis.RegisterHandlers(a.services.pranalysis, regCtx)
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
