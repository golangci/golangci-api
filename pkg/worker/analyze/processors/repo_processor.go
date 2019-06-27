package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analyze/resources"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/golangci/golangci-api/pkg/worker/analyze/logger"

	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/repostate"
	"github.com/golangci/golangci-api/pkg/worker/lib/errorutils"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/workspaces"

	"github.com/pkg/errors"
)

type StaticRepoConfig struct {
	RepoFetcher fetchers.Fetcher
	Linters     []linters.Linter
	Runner      linters.Runner
	State       repostate.Storage
	Cfg         config.Config
	Et          apperrors.Tracker
	AwsSess     *session.Session
}

type RepoConfig struct {
	StaticRepoConfig

	Exec executors.Executor
	Wi   workspaces.Installer
	Ec   *experiments.Checker
}

type Repo struct {
	RepoConfig
}

type RepoContext struct {
	Ctx context.Context

	AnalysisGUID string
	Branch       string
	Repo         *github.Repo // TODO: abstract from repo provider

	PrivateAccessToken string

	Log logutil.Log
}

func (ctx *RepoContext) secrets() []string {
	return []string{ctx.PrivateAccessToken, ctx.AnalysisGUID}
}

func NewRepo(cfg *RepoConfig) *Repo {
	return &Repo{
		RepoConfig: *cfg,
	}
}

func (r Repo) Process(ctx *RepoContext) error {
	res := analysisResult{
		buildLog: result.NewLog(nil),
	}

	savedLogger := ctx.Log
	ctx.Log = logger.NewBuildLogger(res.buildLog, ctx.Log)
	err := r.processPanicSafe(ctx, &res)
	ctx.Log = savedLogger

	if errors.Cause(err) == executors.ErrExecutorFail {
		// temporary error, don't show it to user
		return err
	}

	status := errorToStatus(err)
	publicErrorText := buildPublicError(err)
	err = transformError(err)

	r.submitResult(ctx, &res, status, publicErrorText)
	return err
}

func (r Repo) processPanicSafe(ctx *RepoContext, res *analysisResult) (err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			err = &errorutils.InternalError{
				PublicDesc:  "internal error",
				PrivateDesc: fmt.Sprintf("panic occurred: %s, %s", rerr, debug.Stack()),
			}
		}
	}()

	res.buildLog.RunNewGroupVoid("fetch and update analysis status", func(sg *result.StepGroup) {
		sg.AddStep(stepUpdateStatusToProcessing)
		r.updateStatusToProcessing(ctx, res)
	})

	runErr := res.buildLog.RunNewGroup("setup build environment", func(sg *result.StepGroup) error {
		sg.AddStep("start container")
		defer res.addTimingFrom("Start Container", time.Now())

		requirements := resources.BuildExecutorRequirementsForRepo(r.Ec, ctx.Repo)
		if err := r.Exec.Setup(ctx.Ctx, requirements); err != nil {
			return errors.Wrap(err, "failed to setup executor")
		}
		return nil
	})
	if runErr != nil {
		return runErr
	}

	if err := r.prepare(ctx, res); err != nil {
		return errors.Wrap(err, "failed to prepare repo")
	}

	if err := r.analyze(ctx, res); err != nil {
		return errors.Wrap(err, "failed to analyze repo")
	}

	return nil
}

func (r Repo) updateStatusToProcessing(ctx *RepoContext, res *analysisResult) {
	curState, err := r.State.GetState(ctx.Ctx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID)
	if err != nil {
		ctx.Log.Warnf("Can't get current state: %s", err)
		return
	}

	if curState.Status == StatusSentToQueue {
		res.addTimingFrom("In Queue", fromDBTime(curState.CreatedAt))
		curState.Status = StatusProcessing
		if err = r.State.UpdateState(ctx.Ctx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID, curState); err != nil {
			ctx.Log.Warnf("Can't update repo analysis %s state with setting status to 'processing': %s", ctx.AnalysisGUID, err)
		}
	}
}

func (r *Repo) prepare(ctx *RepoContext, res *analysisResult) error {
	defer res.addTimingFrom("Prepare", time.Now())

	fr := buildFetchersRepo(ctx)
	exec, _, err := r.Wi.Setup(ctx.Ctx, res.buildLog, ctx.PrivateAccessToken, fr, "github.com", ctx.Repo.Owner, ctx.Repo.Name)
	if err != nil {
		return errors.Wrap(err, "failed to setup workspace")
	}

	r.Exec = exec
	return nil
}

func (r Repo) analyze(ctx *RepoContext, res *analysisResult) error {
	defer res.addTimingFrom("Analysis", time.Now())

	return res.buildLog.RunNewGroup("analyze", func(sg *result.StepGroup) error {
		lintRes, err := r.Runner.Run(ctx.Ctx, sg, r.Linters, r.Exec)
		if err != nil {
			return err
		}

		res.lintRes = lintRes
		return nil
	})
}

func buildFetchersRepo(ctx *RepoContext) *fetchers.Repo {
	repo := ctx.Repo
	var cloneURL string
	if ctx.PrivateAccessToken != "" {
		cloneURL = fmt.Sprintf("https://%s@github.com/%s/%s.git",
			ctx.PrivateAccessToken, repo.Owner, repo.Name)
	} else {
		cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", repo.Owner, repo.Name)
	}

	return &fetchers.Repo{
		CloneURL: cloneURL,
		Ref:      ctx.Branch,
		FullPath: fmt.Sprintf("github.com/%s/%s", repo.Owner, repo.Name),
	}
}

func (r Repo) submitResult(ctx *RepoContext, res *analysisResult, status, publicError string) {
	if res.buildLog != nil {
		escapeBuildLog(res.buildLog, ctx)
	}

	resJSON := &resultJSON{
		Version: 1,
		WorkerRes: workerRes{
			Timings:  res.timings,
			Warnings: res.warnings,
			Error:    publicError,
		},
		BuildLog: res.buildLog,
	}

	if res.lintRes != nil {
		resJSON.GolangciLintRes = res.lintRes.ResultJSON
	}
	s := &repostate.State{
		Status:     status,
		ResultJSON: resJSON,
	}

	jsonBytes, err := json.Marshal(*resJSON)
	if err != nil {
		ctx.Log.Warnf("Failed to marshal json: %s", err)
	}

	updateCtx := context.Background() // no timeout for state and status saving: it must be durable
	if err := r.State.UpdateState(updateCtx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID, s); err != nil {
		ctx.Log.Warnf("Can't set analysis %s status to '%s': %s", ctx.AnalysisGUID, string(jsonBytes), err)
		return
	}

	ctx.Log.Infof("Saved repo analysis status: %s", status)
}
