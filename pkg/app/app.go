package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/pkg/app/analyzes/pranalyzes"
	repoanalyzeslib "github.com/golangci/golangci-api/pkg/app/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/app/auth/oauth"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/services/auth"
	"github.com/golangci/golangci-api/pkg/app/services/events"
	"github.com/golangci/golangci-api/pkg/app/services/organization"
	"github.com/golangci/golangci-api/pkg/app/services/pranalysis"
	"github.com/golangci/golangci-api/pkg/app/services/repo"
	"github.com/golangci/golangci-api/pkg/app/services/repoanalysis"
	"github.com/golangci/golangci-api/pkg/app/services/repohook"
	"github.com/golangci/golangci-api/pkg/app/services/subscription"
	"github.com/golangci/golangci-api/pkg/app/utils"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue/repos"
	"github.com/golangci/golangci-api/pkg/cache"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/db/migrations"
	"github.com/golangci/golangci-api/pkg/db/redis"
	"github.com/golangci/golangci-api/pkg/queue/aws/consumer"
	"github.com/golangci/golangci-api/pkg/queue/aws/sqs"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	apisession "github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-api/pkg/transportutil"
	"github.com/golangci/golangci-shared/pkg/apperrors"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/golangci/golangci-worker/app/lib/queue"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/mattes/migrate/database/postgres" // must be first
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	"gopkg.in/redsync.v1"
)

type appServices struct {
	repoanalysis repoanalysis.Service
	repo         repo.Service
	repohook     repohook.Service
	pranalysis   pranalysis.Service
	events       events.Service
	auth         auth.Service
	organisation organization.Service
	subscription subscription.Service
}

type queues struct {
	primarySQS    *sqs.Queue
	primaryDLQSQS *sqs.Queue

	producers struct {
		primaryMultiplexer *producers.Multiplexer

		repoAnalyzesLauncher *repoanalyzes.LauncherProducer
	}
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
	authSessFactory  *apisession.Factory
	providerFactory  providers.Factory
	distLockFactory  *redsync.Redsync
	redisPool        *redigo.Pool

	PRAnalyzesRestarter   *pranalyzes.Restarter // TODO: make private
	repoAnalyzesRestarter *repoanalyzeslib.Restarter
}

func (a App) GetDB() *gorm.DB { // TODO: remove
	return a.gormDB
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
			gormDB, err := gormdb.GetDB(a.cfg, a.trackedLog, dbConnString)
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

	if a.providerFactory == nil {
		a.providerFactory = providers.NewBasicFactory(a.trackedLog)
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

	a.queues.producers.primaryMultiplexer = producers.NewMultiplexer(a.queues.primarySQS)

	repoAnalyzesLauncher := &repoanalyzes.LauncherProducer{}
	if err := repoAnalyzesLauncher.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'launch repo analysis' producer: %s", err)
	}
	a.queues.producers.repoAnalyzesLauncher = repoAnalyzesLauncher
}

func (a *App) buildServices() {
	a.services.repoanalysis = repoanalysis.BasicService{}
	a.services.repohook = repohook.BasicService{
		ProviderFactory:       a.providerFactory,
		AnalysisLauncherQueue: a.queues.producers.repoAnalyzesLauncher,
	}
	a.services.pranalysis = pranalysis.BasicService{}
	a.services.events = events.BasicService{}

	sf, err := apisession.NewFactory(a.redisPool, a.cfg, time.Hour)
	if err != nil {
		a.log.Fatalf("Can't build oauth session factory: %s", err)
	}
	a.services.auth = auth.BasicService{
		Cfg:             a.cfg,
		OAuthFactory:    oauth.NewFactory(sf, a.trackedLog, a.cfg),
		AuthSessFactory: a.authSessFactory,
	}
	a.services.organisation = organization.Configure(
		a.providerFactory,
		cache.NewRedis(a.cfg.GetString("REDIS_URL")+"/1"),
		a.cfg,
	)
	a.services.subscription = subscription.Configure()

	a.buildRepoService()
}

func (a *App) buildRepoService() {
	createRepoQP := &repos.CreatorProducer{}
	if err := createRepoQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'create repo' producer: %s", err)
	}
	deleteRepoQP := &repos.DeleterProducer{}
	if err := deleteRepoQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
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

func (a *App) buildAuthSessFactory() {
	authSessFactory, err := apisession.NewFactory(a.redisPool, a.cfg, 365*24*time.Hour) // 1 year
	if err != nil {
		a.log.Fatalf("Failed to make auth session factory: %s", err)
	}
	a.authSessFactory = authSessFactory
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
	a.buildAuthSessFactory()
	a.buildServices()
	a.buildMigrationsRunner()

	a.PRAnalyzesRestarter = &pranalyzes.Restarter{
		DB:              a.gormDB,
		Log:             a.trackedLog,
		ProviderFactory: a.providerFactory,
	}
	a.repoAnalyzesRestarter = &repoanalyzeslib.Restarter{
		DB:  a.gormDB,
		Log: a.trackedLog,
		Cfg: a.cfg,
	}

	return &a
}

func (a App) registerHandlers(r *mux.Router) {
	regCtx := &transportutil.HandlerRegContext{
		Router:          r,
		Log:             a.log,
		ErrTracker:      a.errTracker,
		DB:              a.gormDB,
		AuthSessFactory: a.authSessFactory,
	}
	repoanalysis.RegisterHandlers(a.services.repoanalysis, regCtx)
	repo.RegisterHandlers(a.services.repo, regCtx)
	repohook.RegisterHandlers(a.services.repohook, regCtx)
	pranalysis.RegisterHandlers(a.services.pranalysis, regCtx)
	events.RegisterHandlers(a.services.events, regCtx)
	auth.RegisterHandlers(a.services.auth, regCtx)
	organization.RegisterHandlers(a.services.organisation, regCtx)
	subscription.RegisterHandlers(a.services.subscription, regCtx)
}

func (a App) runMigrations() {
	if err := a.migrationsRunner.Run(); err != nil {
		a.log.Fatalf("Can't run migrations: %s", err)
	}
}

func (a App) buildMultiplexedConsumer() *consumers.Multiplexer {
	primaryQueueConsumerMultiplexer := consumers.NewMultiplexer()

	repoCreatorConsumer := repos.NewCreatorConsumer(a.trackedLog, a.sqlDB, a.cfg,
		a.providerFactory, a.queues.producers.repoAnalyzesLauncher)
	if err := repoCreatorConsumer.Register(primaryQueueConsumerMultiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo creator consumer: %s", err)
	}
	repoDeleterConsumer := repos.NewDeleterConsumer(a.trackedLog, a.sqlDB, a.cfg, a.providerFactory)
	if err := repoDeleterConsumer.Register(primaryQueueConsumerMultiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo deleter consumer: %s", err)
	}

	analyzesLauncherConsumer := repoanalyzes.NewLauncherConsumer(a.trackedLog, a.sqlDB)
	if err := analyzesLauncherConsumer.Register(primaryQueueConsumerMultiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register analyzes launcher consumer: %s", err)
	}

	return primaryQueueConsumerMultiplexer
}

func (a App) runConsumers() {
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

func (a App) RunEnvironment() {
	queue.Init()
	a.runMigrations()
	a.runConsumers()

	go a.PRAnalyzesRestarter.Run()
	go a.repoAnalyzesRestarter.Run()
}

func (a App) RunForever() {
	a.RunEnvironment()

	http.Handle("/", a.GetHTTPHandler())

	addr := fmt.Sprintf(":%d", a.cfg.GetInt("port", 3000))
	a.log.Infof("Listening on %s...", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		a.log.Errorf("Can't listen HTTP on %s: %s", addr, err)
		os.Exit(1)
	}
}

func (a App) GetHTTPHandler() http.Handler {
	r := mux.NewRouter()
	a.registerHandlers(r)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://golangci.com", "https://dev.golangci.com"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
	})

	n := negroni.Classic()
	n.Use(c)
	n.UseHandler(r)
	return n
}
