package app

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	redigo "github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/redis"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/queue/aws/consumer"
	"github.com/golangci/golangci-api/internal/shared/queue/aws/sqs"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	analyzesConsumers "github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/consumers"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/pullanalyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/repoanalyzesqueue"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	redsync "gopkg.in/redsync.v1"
)

type App struct {
	log             logutil.Log
	trackedLog      logutil.Log
	errTracker      apperrors.Tracker
	cfg             config.Config
	redisPool       *redigo.Pool
	distLockFactory *redsync.Redsync
	awsSess         *session.Session
	ppf             processors.PullProcessorFactory
}

func NewApp(modifiers ...Modifier) *App {
	var a App
	for _, modifier := range modifiers {
		modifier(&a)
	}

	a.buildDeps()
	a.buildAwsSess()

	return &a
}

func (a *App) buildDeps() {
	if a.log == nil {
		slog := logutil.NewStderrLog("golangci-worker")
		slog.SetLevel(logutil.LogLevelInfo)
		a.log = slog
	}

	if a.cfg == nil {
		a.cfg = config.NewEnvConfig(a.log)
	}

	if a.errTracker == nil {
		a.errTracker = apperrors.GetTracker(a.cfg, a.log, "worker")
	}
	if a.trackedLog == nil {
		a.trackedLog = apperrors.WrapLogWithTracker(a.log, nil, a.errTracker)
	}
	if a.redisPool == nil {
		redisPool, err := redis.GetPool(a.cfg)
		if err != nil {
			a.log.Fatalf("Can't get redis pool: %s", err)
		}
		a.redisPool = redisPool
	}
	if a.distLockFactory == nil {
		a.distLockFactory = redsync.New([]redsync.Pool{a.redisPool})
	}
	if a.ppf == nil {
		a.ppf = processors.NewBasicPullProcessorFactory(&processors.BasicPullConfig{})
	}
}

func (a App) buildMultiplexer() *consumers.Multiplexer {
	rpf := processors.NewRepoProcessorFactory(&processors.StaticRepoConfig{})
	repoAnalyzer := analyzesConsumers.NewAnalyzeRepo(rpf, a.trackedLog, a.cfg)
	repoAnalyzesRunner := repoanalyzesqueue.NewConsumer(repoAnalyzer)

	pullAnalyzer := analyzesConsumers.NewAnalyzePR(a.ppf, a.trackedLog)
	pullAnalyzesRunner := pullanalyzesqueue.NewConsumer(pullAnalyzer)

	multiplexer := consumers.NewMultiplexer()

	if err := repoAnalyzesRunner.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register repo analyzer consumer: %s", err)
	}
	if err := pullAnalyzesRunner.Register(multiplexer, a.distLockFactory); err != nil {
		a.log.Fatalf("Failed to register pull analyzer consumer: %s", err)
	}

	return multiplexer
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

type TestDeps struct {
	PullAnalyzesRunner *pullanalyzesqueue.Producer
}

func (a App) BuildTestDeps() *TestDeps {
	analyzesSQS := sqs.NewQueue(a.cfg.GetString("SQS_ANALYZES_QUEUE_URL"),
		a.awsSess, a.trackedLog, analyzesqueue.VisibilityTimeoutSec)
	analyzesMultiplexer := producers.NewMultiplexer(analyzesSQS)

	pullAnalyzesRunner := &pullanalyzesqueue.Producer{}
	if err := pullAnalyzesRunner.Register(analyzesMultiplexer); err != nil {
		a.log.Fatalf("Failed to create 'run pull analysis' producer: %s", err)
	}

	return &TestDeps{
		PullAnalyzesRunner: pullAnalyzesRunner,
	}
}

func (a App) Run() {
	consumerMultiplexer := a.buildMultiplexer()
	analyzesSQS := sqs.NewQueue(a.cfg.GetString("SQS_ANALYZES_QUEUE_URL"),
		a.awsSess, a.trackedLog, analyzesqueue.VisibilityTimeoutSec)
	consumer := consumer.NewSQS(a.trackedLog, a.cfg, analyzesSQS,
		consumerMultiplexer, "analyzes", analyzesqueue.VisibilityTimeoutSec)

	consumer.Run()
}
