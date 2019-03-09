package processors

import (
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/redis"
	redsync "gopkg.in/redsync.v1"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/golinters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/prstate"
	"github.com/golangci/golangci-api/pkg/worker/analyze/reporters"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/workspaces"
	"github.com/golangci/golangci-api/pkg/worker/lib/httputils"
)

type BasicPullProcessorFactory struct {
	cfg *BasicPullConfig
}

func NewBasicPullProcessorFactory(cfg *BasicPullConfig) *BasicPullProcessorFactory {
	return &BasicPullProcessorFactory{
		cfg: cfg,
	}
}

//nolint:gocyclo
func (pf BasicPullProcessorFactory) BuildProcessor(ctx *PullContext) (PullProcessor, func(), error) {
	cfg := *pf.cfg

	if cfg.Cfg == nil {
		cfg.Cfg = config.NewEnvConfig(ctx.Log)
	}

	if cfg.ProviderClient == nil {
		cfg.ProviderClient = github.NewMyClient()
	}

	var cleanup func()

	if cfg.Exec == nil {
		exec, err := makeExecutor(ctx.Ctx, nil)
		if err != nil {
			return nil, nil, errors.Wrap(err, "can't make executor")
		}
		cfg.Exec = exec

		cleanup = func() {
			exec.Clean()
		}
	}

	if cfg.Ec == nil {
		cfg.Ec = experiments.NewChecker(cfg.Cfg, ctx.Log)
	}

	if cfg.RepoFetcher == nil {
		cfg.RepoFetcher = fetchers.NewGit()
	}

	if cfg.Wi == nil {
		cfg.Wi = workspaces.NewGo(cfg.Exec, ctx.Log, cfg.RepoFetcher)
	}

	if cfg.Linters == nil {
		cfg.Linters = []linters.Linter{
			golinters.GolangciLint{
				PatchPath: patchPath,
			},
		}
	}

	if cfg.Reporter == nil {
		cfg.Reporter = reporters.NewGithubReviewer(ctx.ProviderCtx, cfg.ProviderClient, cfg.Ec)
	}

	if cfg.Runner == nil {
		cfg.Runner = linters.SimpleRunner{}
	}

	if cfg.State == nil {
		cfg.State = prstate.NewAPIStorage(httputils.NewGrequestsClient(map[string]string{
			"X-Internal-Access-Token": cfg.Cfg.GetString("INTERNAL_ACCESS_TOKEN"),
		}))
	}

	if cfg.DistLockFactory == nil {
		redisPool, err := redis.GetPool(cfg.Cfg)
		if err != nil {
			ctx.Log.Fatalf("Can't get redis pool: %s", err)
		}
		cfg.DistLockFactory = redsync.New([]redsync.Pool{redisPool})
	}

	return NewBasicPull(&cfg), cleanup, nil
}
