package processors

import (
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
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
	cfg      *StaticRepoConfig
	noCtxLog logutil.Log
}

func NewRepoProcessorFactory(cfg *StaticRepoConfig, noCtxLog logutil.Log) *RepoProcessorFactory {
	return &RepoProcessorFactory{
		cfg:      cfg,
		noCtxLog: noCtxLog,
	}
}

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
		cfg.State = repostate.NewAPIStorage(httputils.GrequestsClient{})
	}

	if cfg.Cfg == nil {
		envCfg := config.NewEnvConfig(f.noCtxLog)
		cfg.Cfg = envCfg
	}

	if cfg.Et == nil {
		cfg.Et = apperrors.GetTracker(cfg.Cfg, f.noCtxLog, "worker")
	}

	lctx := logutil.Context{
		"branch":       ctx.Branch,
		"analysisGUID": ctx.AnalysisGUID,
		"provider":     "github",
		"repoName":     ctx.Repo.FullName(),
		"analysisType": "repo",
	}
	log := logutil.WrapLogWithContext(f.noCtxLog, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, cfg.Et)

	ec := experiments.NewChecker(cfg.Cfg, log)

	exec, err := makeExecutor(ctx.Ctx, ctx.Repo, false, log, ec)
	if err != nil {
		return nil, nil, errors.Wrap(err, "can't make executor")
	}

	cleanup := func() {
		exec.Clean()
	}
	p := NewRepo(&RepoConfig{
		StaticRepoConfig: cfg,
		Log:              log,
		Exec:             exec,
		Wi:               workspaces.NewGo2(exec, log, cfg.RepoFetcher),
		Ec:               ec,
	})

	return p, cleanup, nil
}
