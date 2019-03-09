package processors

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/logger"
	"github.com/golangci/golangci-api/pkg/worker/analyze/prstate"
	"github.com/golangci/golangci-api/pkg/worker/analyze/reporters"
	"github.com/golangci/golangci-api/pkg/worker/lib/errorutils"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/workspaces"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"

	"github.com/golangci/golangci-api/internal/shared/config"
	envbuildresult "github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
)

const (
	patchPath = "../changes.patch"
)

type StaticBasicPullConfig struct {
	RepoFetcher     fetchers.Fetcher
	Linters         []linters.Linter
	Runner          linters.Runner
	ProviderClient  github.Client
	State           prstate.Storage
	Cfg             config.Config
	DistLockFactory *redsync.Redsync
}

type BasicPullConfig struct {
	StaticBasicPullConfig

	Reporter reporters.Reporter
	Exec     executors.Executor
	Wi       workspaces.Installer
	Ec       *experiments.Checker
}

type BasicPull struct {
	BasicPullConfig
}

func NewBasicPull(cfg *BasicPullConfig) *BasicPull {
	return &BasicPull{
		BasicPullConfig: *cfg,
	}
}

func storePatch(ctx context.Context, patch string, exec executors.Executor) error {
	f, err := ioutil.TempFile("/tmp", "golangci.diff")
	defer os.Remove(f.Name())

	if err != nil {
		return fmt.Errorf("can't create temp file for patch: %s", err)
	}
	if err = ioutil.WriteFile(f.Name(), []byte(patch), os.ModePerm); err != nil {
		return fmt.Errorf("can't write patch to temp file %s: %s", f.Name(), err)
	}

	if err = exec.CopyFile(ctx, patchPath, f.Name()); err != nil {
		return fmt.Errorf("can't copy patch file: %s", err)
	}

	return nil
}

func (p BasicPull) getRepo(ctx *PullContext) *fetchers.Repo {
	repo := ctx.ProviderCtx.Repo
	return &fetchers.Repo{
		CloneURL: ctx.ProviderCtx.GetCloneURL(ctx.pull.GetHead().GetRepo()),
		Ref:      ctx.pull.GetHead().GetRef(),
		FullPath: fmt.Sprintf("github.com/%s/%s", repo.Owner, repo.Name),
	}
}

func (p BasicPull) updateAnalysisState(ctx *PullContext, res *result.Result, status github.Status, publicError string) {
	publicError = escapeText(publicError, ctx)

	if ctx.res.buildLog != nil {
		escapeBuildLog(ctx.res.buildLog, ctx)
	}

	resJSON := &resultJSON{
		Version: 1,
		WorkerRes: workerRes{
			Timings:  ctx.res.timings,
			Warnings: ctx.res.warnings,
			Error:    publicError,
		},
		BuildLog: ctx.res.buildLog,
	}

	issuesCount := 0
	if res != nil {
		resJSON.GolangciLintRes = res.ResultJSON
		issuesCount = len(res.Issues)
	}
	s := &prstate.State{
		Status:              "processed/" + string(status),
		ReportedIssuesCount: issuesCount,
		ResultJSON:          resJSON,
	}

	repo := ctx.ProviderCtx.Repo
	if err := p.State.UpdateState(ctx.Ctx, repo.Owner, repo.Name, ctx.AnalysisGUID, s); err != nil {
		ctx.Log.Warnf("Can't set analysis %s status to '%v': %s", ctx.AnalysisGUID, s, err)
	}
}

func (p *BasicPull) processWithGuaranteedGithubStatus(ctx *PullContext) error {
	res, err := p.analyze(ctx)

	ctx.Log = ctx.savedLog
	ctx.savedLog = nil
	ctx.Log.Infof("timings: %s", ctx.res.timings)

	ctx.Ctx = context.Background() // no timeout for state and status saving: it must be durable

	status, statusDesc := pullErrorToGithubStatusAndDesc(err, res)
	publicError := buildPublicError(err)
	err = transformError(err)

	// update of state must be before commit status update: user can open details link before: race condition
	p.updateAnalysisState(ctx, res, status, publicError)
	p.setCommitStatus(ctx, status, statusDesc)

	return err
}

func (p BasicPull) checkPull(ctx *PullContext) error {
	return ctx.res.buildLog.RunNewGroup("check pull request", func(sg *envbuildresult.StepGroup) error {
		sg.AddStep("check state")
		prState := strings.ToUpper(ctx.pull.GetState())
		ctx.Log.Infof("Pull request state is %s", prState)
		if prState != "MERGED" && prState != "CLOSED" {
			return nil
		}

		// branch can be deleted: will be an error; no need to analyze
		ctx.res.publicWarn("process", fmt.Sprintf("Pull Request is already %s, skip analysis", prState))
		ctx.Log.Infof("Pull Request is already %s, skip analysis", prState)
		return &IgnoredError{
			Status:     github.StatusSuccess,
			StatusDesc: fmt.Sprintf("Pull Request is already %s", strings.ToLower(prState)),
		}
	})
}

func (p *BasicPull) analyze(ctx *PullContext) (*result.Result, error) {
	if err := p.checkPull(ctx); err != nil {
		return nil, err
	}

	var res *result.Result
	var err error
	ctx.res.trackTiming("Analysis", func() {
		ctx.res.buildLog.RunNewGroupVoid("analyze", func(sg *envbuildresult.StepGroup) {
			res, err = p.Runner.Run(ctx.Ctx, sg, p.Linters, p.Exec)
		})
	})
	if err != nil {
		return nil, err
	}

	issues := res.Issues
	ctx.LogCtx["reportedIssues"] = len(issues)

	lock := p.DistLockFactory.NewMutex(ctx.ProviderCtx.Repo.FullName())
	if err = lock.Lock(); err != nil {
		return nil, errors.Wrapf(err, "failed to acquire lock %s", ctx.ProviderCtx.Repo.FullName())
	}
	defer lock.Unlock()

	if err = p.Reporter.Report(ctx.Ctx, ctx.buildConfig, ctx.res.buildLog, ctx.pull.GetHead().GetSHA(), issues); err != nil {
		if errors.Cause(err) == github.ErrUserIsBlocked {
			return nil, &errorutils.InternalError{
				PublicDesc:  fmt.Sprintf("@%s is blocked in the organization", p.Cfg.GetString("GITHUB_REVIEWER_LOGIN")),
				PrivateDesc: fmt.Sprintf("can't send pull request comments to github: %s", err),
				IsPermanent: true,
			}
		}

		if errors.Cause(err) == github.ErrCommitIsNotPartOfPull {
			return nil, &errorutils.InternalError{
				PublicDesc:  github.ErrCommitIsNotPartOfPull.Error(),
				PrivateDesc: fmt.Sprintf("can't send pull request comments to github: %s", err),
				IsPermanent: true,
			}
		}

		return nil, &errorutils.InternalError{
			PublicDesc:  "can't send pull request comments to github",
			PrivateDesc: fmt.Sprintf("can't send pull request comments to github: %s", err),
		}
	}

	return res, nil
}

func (p BasicPull) setCommitStatus(ctx *PullContext, status github.Status, desc string) {
	desc = escapeText(desc, ctx)

	var url string
	if status == github.StatusFailure || status == github.StatusSuccess || status == github.StatusError {
		c := ctx.ProviderCtx
		url = fmt.Sprintf("%s/r/github.com/%s/%s/pulls/%d",
			p.Cfg.GetString("WEB_ROOT"), c.Repo.Owner, c.Repo.Name, ctx.pull.GetNumber())
	}

	err := p.ProviderClient.SetCommitStatus(ctx.Ctx, ctx.ProviderCtx, ctx.pull.GetHead().GetSHA(), status, desc, url)
	if err != nil {
		ctx.res.publicWarn("github", "Can't set VCS provider commit status") // TODO: write provider name
		ctx.Log.Warnf("Can't set provider commit status: %s", err)
	}
}

func (p BasicPull) preparePatch(ctx *PullContext) error {
	return ctx.res.buildLog.RunNewGroup("prepare pull request patch", func(sg *envbuildresult.StepGroup) error {
		sg.AddStep("fetch patch from VCS provider")
		patch, err := p.ProviderClient.GetPullRequestPatch(ctx.Ctx, ctx.ProviderCtx)
		if err != nil {
			return errors.Wrap(err, "can't get patch")
		}

		sg.AddStep("copy patch to /tmp/golangci.diff")
		if err = storePatch(ctx.Ctx, patch, p.Exec); err != nil {
			return errors.Wrap(err, "can't store patch")
		}

		return nil
	})
}

func (p *BasicPull) setupWorkspace(ctx *PullContext) error {
	privateAccessToken := ""
	if ctx.ProviderCtx.Repo.IsPrivate {
		privateAccessToken = ctx.ProviderCtx.GithubAccessToken
	}
	exec, buildConfig, err := p.Wi.Setup(ctx.Ctx, ctx.res.buildLog, privateAccessToken,
		p.getRepo(ctx), "github.com", ctx.repo().Owner, ctx.repo().Name) //nolint:govet
	if err != nil {
		publicError := fmt.Sprintf("failed to setup workspace: %s", err)
		p.updateAnalysisState(ctx, nil, github.StatusError, publicError)
		p.setCommitStatus(ctx, github.StatusError, "failed to setup")
		return errors.Wrapf(err, "failed to setup workspace")
	}

	p.Exec = exec
	ctx.buildConfig = buildConfig
	return nil
}

func (p *BasicPull) fetchProviderPullRequest(ctx *PullContext) error {
	pull, err := p.ProviderClient.GetPullRequest(ctx.Ctx, ctx.ProviderCtx)
	if err != nil {
		return errors.Wrap(err, "can't get pull request")
	}

	ctx.pull = pull
	return nil
}

func (p *BasicPull) updateStatusToProcessing(ctx *PullContext) {
	p.setCommitStatus(ctx, github.StatusPending, "GolangCI is reviewing your Pull Request...")

	curState, err := p.State.GetState(ctx.Ctx, ctx.repo().Owner, ctx.repo().Name, ctx.AnalysisGUID)
	if err != nil {
		ctx.Log.Warnf("Can't get current state: %s", err)
		return
	}

	if curState.Status != StatusSentToQueue {
		return
	}

	ctx.res.addTimingFrom("In Queue", fromDBTime(curState.CreatedAt))
	inQueue := time.Since(fromDBTime(curState.CreatedAt))
	ctx.LogCtx["inQueueSeconds"] = int(inQueue / time.Second)
	curState.Status = StatusProcessing
	if err = p.State.UpdateState(ctx.Ctx, ctx.repo().Owner, ctx.repo().Name, ctx.AnalysisGUID, curState); err != nil {
		ctx.Log.Warnf("Can't update analysis %s state with setting status to 'processing': %s", ctx.AnalysisGUID, err)
	}
}

func (p BasicPull) Process(ctx *PullContext) error {
	ctx.res = &analysisResult{
		buildLog: envbuildresult.NewLog(nil),
	}

	savedLog := ctx.Log
	ctx.savedLog = ctx.Log
	ctx.Log = logger.NewBuildLogger(ctx.res.buildLog, ctx.Log)

	if err := p.processPanicSafe(ctx); err != nil {
		if ctx.pull != nil {
			pullTitle := strings.ToLower(ctx.pull.GetTitle())
			if strings.HasPrefix(pullTitle, "wip ") || strings.HasPrefix(pullTitle, "wip:") {
				savedLog.Infof("Analyze of WIP PR failed, don't retry: %s", err)
				return nil
			}
		}
		return err
	}

	return nil
}

func (p BasicPull) processPanicSafe(ctx *PullContext) (retErr error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			retErr = &errorutils.InternalError{
				PublicDesc:  "internal error",
				PrivateDesc: fmt.Sprintf("panic occured: %s, %s", rerr, debug.Stack()),
			}
			// TODO: set github commit status to internal error
		}
	}()

	groupErr := ctx.res.buildLog.RunNewGroup("prepare service", func(sg *envbuildresult.StepGroup) error {
		sg.AddStep("fetch VCS provider pull request")
		if err := p.fetchProviderPullRequest(ctx); err != nil {
			return err
		}

		sg.AddStep(stepUpdateStatusToProcessing)
		p.updateStatusToProcessing(ctx)
		return nil
	})
	if groupErr != nil {
		return groupErr
	}

	startedAt := time.Now()
	if err := p.setupWorkspace(ctx); err != nil {
		return err
	}

	if err := p.preparePatch(ctx); err != nil {
		return err
	}

	ctx.res.addTimingFrom("Prepare", startedAt)

	return p.processWithGuaranteedGithubStatus(ctx)
}
