package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/api/policy"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/pullanalyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/repoanalyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/internal/api/endpointutil"
	"github.com/golangci/golangci-api/internal/api/paymentproviders"
	apisession "github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/db/migrations"
	"github.com/golangci/golangci-api/internal/shared/db/redis"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/queue/aws/consumer"
	"github.com/golangci/golangci-api/internal/shared/queue/aws/sqs"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	apiauth "github.com/golangci/golangci-api/pkg/api/auth"
	"github.com/golangci/golangci-api/pkg/api/auth/oauth"
	"github.com/golangci/golangci-api/pkg/api/crons/pranalyzes"
	"github.com/golangci/golangci-api/pkg/api/crons/repoinfo"
	"github.com/golangci/golangci-api/pkg/api/services/auth"
	"github.com/golangci/golangci-api/pkg/api/services/events"
	"github.com/golangci/golangci-api/pkg/api/services/organization"
	"github.com/golangci/golangci-api/pkg/api/services/pranalysis"
	"github.com/golangci/golangci-api/pkg/api/services/repo"
	"github.com/golangci/golangci-api/pkg/api/services/repoanalysis"
	"github.com/golangci/golangci-api/pkg/api/services/repohook"
	"github.com/golangci/golangci-api/pkg/api/services/subscription"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/invitations"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/paymentevents"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/repos"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/subs"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/mattes/migrate/database/postgres" // must be first
	"github.com/rs/cors"
	"github.com/urfave/negroni"
	redsync "gopkg.in/redsync.v1"
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

	analyzesSQS *sqs.Queue

	producers struct {
		primaryMultiplexer  *producers.Multiplexer
		analyzesMultiplexer *producers.Multiplexer

		repoAnalyzesLauncher *repoanalyzes.LauncherProducer
		repoAnalyzesRunner   *repoanalyzesqueue.Producer
		pullAnalyzesRunner   *pullanalyzesqueue.Producer
	}
}

type policies struct {
	org       *policy.Organization
	activeSub *policy.ActiveSubscription
	repo      *policy.Repo
}

type App struct {
	cfg                    config.Config
	log                    logutil.Log
	trackedLog             logutil.Log
	errTracker             apperrors.Tracker
	gormDB                 *gorm.DB
	sqlDB                  *sql.DB
	migrationsRunner       *migrations.Runner
	services               appServices
	awsSess                *session.Session
	queues                 queues
	authorizer             *apiauth.Authorizer
	providerFactory        providers.Factory
	paymentProviderFactory paymentproviders.Factory
	distLockFactory        *redsync.Redsync
	redisPool              *redigo.Pool
	ec                     *experiments.Checker
	policies               policies
	cache                  cache.Cache

	PRAnalyzesStaler *pranalyzes.Staler // TODO: make private
	repoInfoUpdater  *repoinfo.Updater
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

	if a.paymentProviderFactory == nil {
		a.paymentProviderFactory = paymentproviders.NewBasicFactory(a.trackedLog, a.cfg)
	}

	if a.redisPool == nil {
		redisPool, err := redis.GetPool(a.cfg)
		if err != nil {
			a.log.Fatalf("Can't get redis pool: %s", err)
		}
		a.redisPool = redisPool
	}

	if a.ec == nil {
		a.ec = experiments.NewChecker(a.cfg, a.trackedLog)
	}

	if a.cache == nil {
		redisURL, err := redis.GetURL(a.cfg)
		if err != nil {
			a.log.Fatalf("Can't get redis URL: %s", err)
		}
		a.cache = cache.NewRedis(redisURL + "/1")
	}

	if a.authorizer == nil {
		authSessFactory, err := apisession.NewFactory(a.redisPool, a.cfg, 365*24*time.Hour) // 1 year
		if err != nil {
			a.log.Fatalf("Failed to make auth session factory: %s", err)
		}

		a.authorizer = apiauth.NewAuthorizer(a.gormDB, authSessFactory)
	}

	if a.policies.org == nil {
		a.policies.org = policy.NewOrganization(a.providerFactory, a.cache, a.cfg, a.trackedLog)
	}
	if a.policies.activeSub == nil {
		a.policies.activeSub = policy.NewActiveSubscription(a.trackedLog, a.gormDB, a.cfg)
	}
	if a.policies.repo == nil {
		a.policies.repo = policy.NewRepo(a.providerFactory, a.cfg, a.trackedLog, a.cache, a.authorizer)
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

func (a App) getQueueURL(name string, required bool) string {
	key := fmt.Sprintf("SQS_%s_QUEUE_URL", strings.ToUpper(name))
	url := a.cfg.GetString(key)
	if url == "" {
		logger := a.log.Infof
		if required {
			logger = a.log.Fatalf
		}
		logger("Failed to get sqs %s queue url: no config key %s", name, key)
	}

	return url
}

func (a *App) buildQueues() {
	a.queues.primarySQS = sqs.NewQueue(a.getQueueURL("primary", true),
		a.awsSess, a.trackedLog, primaryqueue.VisibilityTimeoutSec)
	a.queues.analyzesSQS = sqs.NewQueue(a.getQueueURL("analyzes", true),
		a.awsSess, a.trackedLog, analyzesqueue.VisibilityTimeoutSec)
	a.queues.primaryDLQSQS = sqs.NewQueue(a.getQueueURL("primarydeadletter", false),
		a.awsSess, a.trackedLog, primaryqueue.VisibilityTimeoutSec)

	a.queues.producers.primaryMultiplexer = producers.NewMultiplexer(a.queues.primarySQS)
	a.queues.producers.analyzesMultiplexer = producers.NewMultiplexer(a.queues.analyzesSQS)

	repoAnalyzesRunner := &repoanalyzesqueue.Producer{}
	if err := repoAnalyzesRunner.Register(a.queues.producers.analyzesMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'run repo analysis' producer: %s", err)
	}
	a.queues.producers.repoAnalyzesRunner = repoAnalyzesRunner

	pullAnalyzesRunner := &pullanalyzesqueue.Producer{}
	if err := pullAnalyzesRunner.Register(a.queues.producers.analyzesMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'run pull analysis' producer: %s", err)
	}
	a.queues.producers.pullAnalyzesRunner = pullAnalyzesRunner

	repoAnalyzesLauncher := &repoanalyzes.LauncherProducer{}
	if err := repoAnalyzesLauncher.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'launch repo analysis' producer: %s", err)
	}
	a.queues.producers.repoAnalyzesLauncher = repoAnalyzesLauncher
}

func (a *App) buildServices() {
	a.services.repoanalysis = repoanalysis.BasicService{
		RepoPolicy: a.policies.repo,
	}
	a.services.repohook = repohook.BasicService{
		ProviderFactory:       a.providerFactory,
		AnalysisLauncherQueue: a.queues.producers.repoAnalyzesLauncher,
		PullAnalyzeQueue:      a.queues.producers.pullAnalyzesRunner,
		ActiveSubPolicy:       a.policies.activeSub,
		Cfg:                   a.cfg,
	}
	a.services.pranalysis = pranalysis.BasicService{
		RepoPolicy: a.policies.repo,
		Pf:         a.providerFactory,
		Cfg:        a.cfg,
	}
	a.services.events = events.BasicService{}

	sf, err := apisession.NewFactory(a.redisPool, a.cfg, time.Hour)
	if err != nil {
		a.log.Fatalf("Can't build oauth session factory: %s", err)
	}
	a.services.auth = auth.BasicService{
		Cfg:          a.cfg,
		OAuthFactory: oauth.NewFactory(sf, a.trackedLog, a.cfg),
		Authorizer:   a.authorizer,
	}

	a.services.organisation = organization.NewBasicService(a.policies.org)

	a.buildRepoService()
	a.buildSubService()
}

func (a *App) buildSubService() {
	createSubQP := &subs.CreatorProducer{}
	if err := createSubQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'create sub' producer: %s", err)
	}
	deleteSubQP := &subs.DeleterProducer{}
	if err := deleteSubQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'delete sub' producer: %s", err)
	}
	updateSubQP := &subs.UpdaterProducer{}
	if err := updateSubQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'update sub' producer: %s", err)
	}

	createEventQP := &paymentevents.CreatorProducer{}
	if err := createEventQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'create payment event' producer: %s", err)
	}

	a.services.subscription = subscription.NewBasicService(a.policies.org, a.cfg, updateSubQP, createEventQP)
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
		Cache:           a.cache,
		Ec:              a.ec,
		OrgPolicy:       a.policies.org,
		ActiveSubPolicy: a.policies.activeSub,
	}
}

func (a *App) buildMigrationsRunner() {
	a.distLockFactory = redsync.New([]redsync.Pool{a.redisPool})
	dbConnString, err := gormdb.GetDBConnString(a.cfg)
	if err != nil {
		a.log.Fatalf("Can't get DB conn string: %s", err)
	}
	a.migrationsRunner = migrations.NewRunner(a.distLockFactory.NewMutex("migrations"), a.trackedLog,
		dbConnString, a.cfg.GetString("MIGRATIONS_PATH"))
}

func NewApp(modifiers ...Modifier) *App {
	a := App{}
	for _, m := range modifiers {
		m(&a)
	}
	a.buildDeps()
	a.buildAwsSess()
	a.buildQueues()
	a.buildServices()
	a.buildMigrationsRunner()

	a.PRAnalyzesStaler = &pranalyzes.Staler{
		Cfg:             a.cfg,
		DB:              a.gormDB,
		Log:             a.trackedLog,
		ProviderFactory: a.providerFactory,
	}
	a.repoInfoUpdater = &repoinfo.Updater{
		DB:  a.gormDB,
		Cfg: a.cfg,
		Log: a.trackedLog,
		Pf:  a.providerFactory,
	}

	return &a
}

func (a App) registerHandlers(r *mux.Router) {
	regCtx := &endpointutil.HandlerRegContext{
		Log:        a.log,
		ErrTracker: a.errTracker,
		Cfg:        a.cfg,
		DB:         a.gormDB,
		Authorizer: a.authorizer,
	}
	repoanalysis.RegisterHandlers(a.services.repoanalysis, r, regCtx)
	repo.RegisterHandlers(a.services.repo, r, regCtx)
	repohook.RegisterHandlers(a.services.repohook, r, regCtx)
	pranalysis.RegisterHandlers(a.services.pranalysis, r, regCtx)
	events.RegisterHandlers(a.services.events, r, regCtx)
	auth.RegisterHandlers(a.services.auth, r, regCtx)
	organization.RegisterHandlers(a.services.organisation, r, regCtx)
	subscription.RegisterHandlers(a.services.subscription, r, regCtx)
}

func (a App) runMigrations() {
	if err := a.migrationsRunner.Run(); err != nil {
		a.log.Fatalf("Can't run migrations: %s", err)
	}
}

func (a App) buildMultiplexedPrimaryQueueConsumer() *consumers.Multiplexer {
	multiplexer := consumers.NewMultiplexer()

	acceptInvitationsQP := &invitations.AcceptorProducer{}
	if err := acceptInvitationsQP.Register(a.queues.producers.primaryMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'accept invitations' producer: %s", err)
	}

	repoCreator := repos.NewCreatorConsumer(a.trackedLog, a.sqlDB, a.cfg,
		a.providerFactory, a.queues.producers.repoAnalyzesLauncher, acceptInvitationsQP)
	if err := repoCreator.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo creator consumer: %s", err)
	}
	repoDeleter := repos.NewDeleterConsumer(a.trackedLog, a.sqlDB, a.cfg, a.providerFactory)
	if err := repoDeleter.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo deleter consumer: %s", err)
	}

	subCreator := subs.NewCreatorConsumer(a.trackedLog, a.sqlDB, a.cfg, a.paymentProviderFactory)
	if err := subCreator.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register sub creator consumer: %s", err)
	}

	subUpdater := subs.NewUpdaterConsumer(a.trackedLog, a.sqlDB, a.cfg, a.paymentProviderFactory)
	if err := subUpdater.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register sub updater consumer: %s", err)
	}

	subDeleter := subs.NewDeleterConsumer(a.trackedLog, a.sqlDB, a.cfg, a.paymentProviderFactory)
	if err := subDeleter.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register sub deleter consumer: %s", err)
	}

	paymentEventCreator := paymentevents.NewCreatorConsumer(a.trackedLog, a.sqlDB, a.cfg, a.paymentProviderFactory)
	if err := paymentEventCreator.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register payment event creator consumer: %s", err)
	}

	analyzesLauncher := repoanalyzes.NewLauncherConsumer(a.trackedLog, a.sqlDB,
		a.queues.producers.repoAnalyzesRunner, a.providerFactory)
	if err := analyzesLauncher.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register analyzes launcher consumer: %s", err)
	}

	invitationsAcceptor := invitations.NewAcceptorConsumer(a.trackedLog, a.cfg, a.providerFactory)
	if err := invitationsAcceptor.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register invitations acceptor consumer: %s", err)
	}

	return multiplexer
}

func (a App) runConsumers() {
	primaryQueueConsumerMultiplexer := a.buildMultiplexedPrimaryQueueConsumer()
	primaryQueueConsumer := consumer.NewSQS(a.trackedLog, a.cfg, a.queues.primarySQS,
		primaryQueueConsumerMultiplexer, "primary", primaryqueue.VisibilityTimeoutSec)

	go primaryQueueConsumer.Run()
}

func (a App) RunDeadLetterConsumers() {
	primaryDLQConsumerMultiplexer := a.buildMultiplexedPrimaryQueueConsumer()
	primaryDLQConsumer := consumer.NewSQS(a.trackedLog, a.cfg, a.queues.primaryDLQSQS,
		primaryDLQConsumerMultiplexer, "primaryDeadLetter", primaryqueue.VisibilityTimeoutSec)

	primaryDLQConsumer.Run()
}

func (a App) RecoverAnalyzes() error {
	r := pranalyzes.NewReanalyzer(a.gormDB, a.cfg, a.log, a.providerFactory, a.queues.producers.pullAnalyzesRunner)
	return r.RunOnce()
}

func (a App) PurgeAnalyzesQueue() error {
	return a.queues.analyzesSQS.Purge()
}

func (a App) RunEnvironment() {
	a.runMigrations()
	a.runConsumers()

	go a.PRAnalyzesStaler.Run()
	go a.repoInfoUpdater.Run()
}

func (a App) RunForever() {
	a.RunEnvironment()

	http.Handle("/", a.GetHTTPHandler())

	addr := fmt.Sprintf(":%d", a.cfg.GetInt("PORT", 3000))
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

func (a App) GetRepoAnalyzesRunQueue() *repoanalyzesqueue.Producer {
	return a.queues.producers.repoAnalyzesRunner
}
