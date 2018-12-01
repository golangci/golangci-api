package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenvbuild/ensuredeps"
	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/golinters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/repoinfo"
	"github.com/golangci/golangci-api/pkg/worker/analyze/repostate"
	"github.com/golangci/golangci-api/pkg/worker/lib/errorutils"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/workspaces"
	"github.com/golangci/golangci-api/pkg/worker/lib/httputils"
	"github.com/pkg/errors"
)

type GithubGoRepoConfig struct {
	repoFetcher fetchers.Fetcher
	infoFetcher repoinfo.Fetcher
	linters     []linters.Linter
	runner      linters.Runner
	exec        executors.Executor
	state       repostate.Storage
}

type GithubGoRepo struct {
	analysisGUID string
	branch       string
	gw           *workspaces.Go
	repo         *github.Repo

	GithubGoRepoConfig
	resultCollector
}

func NewGithubGoRepo(ctx context.Context, cfg GithubGoRepoConfig, analysisGUID, repoName, branch string) (*GithubGoRepo, error) {
	parts := strings.Split(repoName, "/")
	repo := &github.Repo{
		Owner: parts[0],
		Name:  parts[1],
	}
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo name %s", repoName)
	}

	if cfg.exec == nil {
		var err error
		cfg.exec, err = makeExecutor(ctx, repo, true, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("can't make executor: %s", err)
		}
	}

	if cfg.repoFetcher == nil {
		cfg.repoFetcher = fetchers.NewGit()
	}

	if cfg.infoFetcher == nil {
		cfg.infoFetcher = repoinfo.NewCloningFetcher(cfg.repoFetcher)
	}

	if cfg.linters == nil {
		cfg.linters = []linters.Linter{
			golinters.GolangciLint{},
		}
	}

	if cfg.runner == nil {
		cfg.runner = linters.SimpleRunner{}
	}

	if cfg.state == nil {
		cfg.state = repostate.NewAPIStorage(httputils.GrequestsClient{})
	}

	return &GithubGoRepo{
		GithubGoRepoConfig: cfg,
		analysisGUID:       analysisGUID,
		branch:             branch,
		repo:               repo,
	}, nil
}

func (g *GithubGoRepo) getRepo() *fetchers.Repo {
	return &fetchers.Repo{
		CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", g.repo.Owner, g.repo.Name),
		Ref:      g.branch,
		FullPath: fmt.Sprintf("github.com/%s/%s", g.repo.Owner, g.repo.Name),
	}
}

func (g *GithubGoRepo) prepareRepo(ctx context.Context) error {
	repo := g.getRepo()
	var err error
	g.trackTiming("Clone", func() {
		err = g.repoFetcher.Fetch(ctx, repo, g.exec)
	})
	if err != nil {
		return &errorutils.InternalError{
			PublicDesc:  "can't clone git repo",
			PrivateDesc: fmt.Sprintf("can't clone git repo: %s", err),
		}
	}

	var depsRes *ensuredeps.Result
	g.trackTiming("Deps", func() {
		depsRes, err = g.gw.FetchDeps(ctx, repo.FullPath)
	})
	if err != nil {
		// don't public warn: it's an internal error
		analytics.Log(ctx).Warnf("Internal error fetching deps: %s", err)
	} else {
		analytics.Log(ctx).Infof("Got deps result: %#v", depsRes)

		//for _, w := range depsRes.Warnings {
		//	warnText := fmt.Sprintf("Fetch deps: %s: %s", w.Kind, w.Text)
		//	warnText = escapeErrorText(warnText, g.buildSecrets())
		//	g.publicWarn("prepare repo", warnText)
		//
		//	analytics.Log(ctx).Infof("Fetch deps warning: [%s]: %s", w.Kind, w.Text)
		//}
	}

	return nil
}

func (g GithubGoRepo) updateAnalysisState(ctx context.Context, res *result.Result, status, publicError string) {
	resJSON := &resultJSON{
		Version: 1,
		WorkerRes: workerRes{
			Timings:  g.timings,
			Warnings: g.warnings,
			Error:    publicError,
		},
	}

	if res != nil {
		resJSON.GolangciLintRes = res.ResultJSON
	}
	s := &repostate.State{
		Status:     status,
		ResultJSON: resJSON,
	}

	jsonBytes, err := json.Marshal(*resJSON)
	if err == nil {
		analytics.Log(ctx).Infof("Save repo analysis status: status=%s, result_json=%s", status, string(jsonBytes))
	}

	if err := g.state.UpdateState(ctx, g.repo.Owner, g.repo.Name, g.analysisGUID, s); err != nil {
		analytics.Log(ctx).Warnf("Can't set analysis %s status to '%v': %s", g.analysisGUID, s, err)
	}
}

func (g *GithubGoRepo) processWithGuaranteedGithubStatus(ctx context.Context) error {
	res, err := g.work(ctx)
	analytics.Log(ctx).Infof("timings: %s", g.timings)

	ctx = context.Background() // no timeout for state and status saving: it must be durable

	var status string
	var publicError string
	if err != nil {
		if ierr, ok := err.(*errorutils.InternalError); ok {
			if strings.Contains(ierr.PrivateDesc, noGoFilesToAnalyzeErr) {
				publicError = noGoFilesToAnalyzeMessage
				status = statusProcessed
				err = nil
			} else {
				status = string(github.StatusError)
				publicError = ierr.PublicDesc
			}
		} else if berr, ok := err.(*errorutils.BadInputError); ok {
			berr.PublicDesc = escapeErrorText(berr.PublicDesc, g.buildSecrets())
			publicError = berr.PublicDesc
			status = statusProcessed
			err = nil
			analytics.Log(ctx).Warnf("Repo analysis bad input error: %s", berr)
		} else {
			status = string(github.StatusError)
			publicError = internalError
		}
	} else {
		status = statusProcessed
	}

	g.updateAnalysisState(ctx, res, status, publicError)
	return err
}

func (g GithubGoRepo) buildSecrets() map[string]string {
	const hidden = "{hidden}"
	ret := map[string]string{
		g.gw.Gopath(): "$GOPATH",
	}

	for _, kv := range os.Environ() {
		parts := strings.Split(kv, "=")
		if len(parts) != 2 {
			continue
		}

		v := parts[1]
		if len(v) >= 6 {
			ret[v] = hidden
		}
	}

	return ret
}

func (g *GithubGoRepo) work(ctx context.Context) (res *result.Result, err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = &errorutils.InternalError{
				PublicDesc:  "golangci-worker panic-ed",
				PrivateDesc: fmt.Sprintf("panic occured: %s, %s", rerr, debug.Stack()),
			}
		}
	}()

	if err = g.prepareRepo(ctx); err != nil {
		return nil, err // don't wrap error, need to save it's type
	}

	g.trackTiming("Analysis", func() {
		res, err = g.runner.Run(ctx, g.linters, g.exec)
	})
	if err != nil {
		return nil, err // don't wrap error, need to save it's type
	}

	return res, nil
}

func (g GithubGoRepo) Process(ctx context.Context) error {
	defer g.exec.Clean()

	curState, err := g.state.GetState(ctx, g.repo.Owner, g.repo.Name, g.analysisGUID)
	if err != nil {
		return fmt.Errorf("can't get current state: %s", err)
	}

	g.gw = workspaces.NewGo(g.exec, g.infoFetcher)
	defer g.gw.Clean(ctx)
	if err = g.gw.Setup(ctx, g.getRepo(), "github.com", g.repo.Owner, g.repo.Name); err != nil {
		if errors.Cause(err) == fetchers.ErrNoBranchOrRepo {
			curState.Status = statusNotFound
			if updateErr := g.state.UpdateState(ctx, g.repo.Owner, g.repo.Name, g.analysisGUID, curState); updateErr != nil {
				analytics.Log(ctx).Warnf("Can't update repo analysis %s state with setting status to 'not_found': %s",
					g.analysisGUID, updateErr)
			}
			analytics.Log(ctx).Warnf("Branch or repo doesn't exist, set status not_found")
			return nil
		}
		return fmt.Errorf("can't setup go workspace: %s", err)
	}
	g.exec = g.gw.Executor()

	if curState.Status == statusSentToQueue {
		g.addTimingFrom("In Queue", fromDBTime(curState.CreatedAt))
		curState.Status = statusProcessing
		if err = g.state.UpdateState(ctx, g.repo.Owner, g.repo.Name, g.analysisGUID, curState); err != nil {
			analytics.Log(ctx).Warnf("Can't update repo analysis %s state with setting status to 'processing': %s", g.analysisGUID, err)
		}
	}

	return g.processWithGuaranteedGithubStatus(ctx)
}
