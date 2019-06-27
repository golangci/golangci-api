package processors

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/golinters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/repostate"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/workspaces"
	"github.com/golangci/golangci-api/pkg/worker/lib/httputils"
	"github.com/pkg/errors"
)

type RepoProcessorFactory struct {
	cfg *StaticRepoConfig
}

func NewRepoProcessorFactory(cfg *StaticRepoConfig) *RepoProcessorFactory {
	return &RepoProcessorFactory{
		cfg: cfg,
	}
}

//nolint:gocyclo
func (f RepoProcessorFactory) BuildProcessor(ctx *RepoContext) (*Repo, func(), error) {
	cfg := *f.cfg

	if cfg.RepoFetcher == nil {
		cfg.RepoFetcher = fetchers.NewGit()
	}

	if cfg.Linters == nil {
		cfg.Linters = []linters.Linter{
			golinters.GolangciLint{},
		}
	}

	if cfg.Runner == nil {
		cfg.Runner = linters.SimpleRunner{}
	}

	if cfg.State == nil {
		cfg.State = repostate.NewAPIStorage(httputils.NewGrequestsClient(map[string]string{
			"X-Internal-Access-Token": os.Getenv("INTERNAL_ACCESS_TOKEN"),
		}))
	}

	if cfg.Cfg == nil {
		envCfg := config.NewEnvConfig(ctx.Log)
		cfg.Cfg = envCfg
	}

	if cfg.Et == nil {
		cfg.Et = apperrors.GetTracker(cfg.Cfg, ctx.Log, "worker")
	}

	if cfg.AwsSess == nil {
		awsCfg := aws.NewConfig().WithRegion("us-east-1")
		if cfg.Cfg.GetBool("AWS_DEBUG", false) {
			awsCfg = awsCfg.WithLogLevel(aws.LogDebugWithHTTPBody)
		}
		awsSess, err := session.NewSession(awsCfg)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to make aws session")
		}
		cfg.AwsSess = awsSess
	}

	ec := experiments.NewChecker(cfg.Cfg, ctx.Log)

	exec, err := makeExecutor(ctx.Log, ec, ctx.Repo, cfg.Cfg, cfg.AwsSess, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "can't make executor")
	}

	cleanup := func() {
		exec.Clean()
	}
	p := NewRepo(&RepoConfig{
		StaticRepoConfig: cfg,
		Exec:             exec,
		Wi:               workspaces.NewGo(exec, ctx.Log, cfg.RepoFetcher),
		Ec:               ec,
	})

	return p, cleanup, nil
}
