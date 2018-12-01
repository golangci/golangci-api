package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	lintersResult "github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
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
}

type RepoConfig struct {
	StaticRepoConfig

	Log  logutil.Log
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
}

type repoResult struct {
	resultCollector
	prepareLog *result.Log
	lintRes    *lintersResult.Result
}

func NewRepo(cfg *RepoConfig) *Repo {
	return &Repo{
		RepoConfig: *cfg,
	}
}

func (r Repo) Process(ctx *RepoContext) {
	res, err := r.processPanicSafe(ctx)
	if res == nil {
		res = &repoResult{}
	}

	r.submitResult(ctx, res, err)
}

func (r Repo) processPanicSafe(ctx *RepoContext) (retRes *repoResult, err error) {
	defer func() {
		if rerr := recover(); rerr != nil {
			retRes = nil
			err = &errorutils.InternalError{
				PublicDesc:  "internal error",
				PrivateDesc: fmt.Sprintf("panic occured: %s, %s", rerr, debug.Stack()),
			}
		}
	}()

	var res repoResult
	r.updateStatusToInQueue(ctx, &res)

	if err := r.prepare(ctx, &res); err != nil {
		return nil, errors.Wrap(err, "failed to prepare repo")
	}

	if err := r.analyze(ctx, &res); err != nil {
		return nil, errors.Wrap(err, "failed to analyze repo")
	}

	return &res, nil
}

func (r Repo) updateStatusToInQueue(ctx *RepoContext, res *repoResult) {
	curState, err := r.State.GetState(ctx.Ctx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID)
	if err != nil {
		r.Log.Warnf("Can't get current state: %s", err)
	} else if curState.Status == statusSentToQueue {
		res.addTimingFrom("In Queue", fromDBTime(curState.CreatedAt))
		curState.Status = statusProcessing
		if err = r.State.UpdateState(ctx.Ctx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID, curState); err != nil {
			r.Log.Warnf("Can't update repo analysis %s state with setting status to 'processing': %s", ctx.AnalysisGUID, err)
		}
	}
}

func (r *Repo) prepare(ctx *RepoContext, res *repoResult) error {
	defer res.addTimingFrom("Prepare", time.Now())

	fr := buildFetchersRepo(ctx)
	exec, resLog, err := r.Wi.Setup(ctx.Ctx, fr, "github.com", ctx.Repo.Owner, ctx.Repo.Name)
	if err != nil {
		return errors.Wrap(err, "failed to setup workspace")
	}

	r.Exec = exec
	res.prepareLog = resLog
	return nil
}

func (r Repo) analyze(ctx *RepoContext, res *repoResult) error {
	defer res.addTimingFrom("Analysis", time.Now())

	lintRes, err := r.Runner.Run(ctx.Ctx, r.Linters, r.Exec)
	if err != nil {
		return errors.Wrap(err, "failed running linters")
	}

	res.lintRes = lintRes
	return nil
}

func buildFetchersRepo(ctx *RepoContext) *fetchers.Repo {
	repo := ctx.Repo
	return &fetchers.Repo{
		CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", repo.Owner, repo.Name),
		Ref:      ctx.Branch,
		FullPath: fmt.Sprintf("github.com/%s/%s", repo.Owner, repo.Name),
	}
}

// TODO: migrate to golangci-lint linter runner when pr processor will have the same code
func (r Repo) transformError(err error) error {
	if err == nil {
		return nil
	}

	causeErr := errors.Cause(err)
	if causeErr == fetchers.ErrNoBranchOrRepo {
		return causeErr
	}

	if ierr, ok := causeErr.(*errorutils.InternalError); ok {
		if strings.Contains(ierr.PrivateDesc, noGoFilesToAnalyzeErr) {
			return errNothingToAnalyze
		}

		return ierr
	}

	return err
}

func (r Repo) errorToStatus(err error) string {
	if err == nil {
		return statusProcessed
	}

	if err == errNothingToAnalyze {
		return statusProcessed
	}

	if _, ok := err.(*errorutils.BadInputError); ok {
		return statusProcessed
	}

	if _, ok := err.(*errorutils.InternalError); ok {
		return string(github.StatusError)
	}

	if err == fetchers.ErrNoBranchOrRepo {
		return statusNotFound
	}

	return string(github.StatusError)
}

func (r Repo) buildPublicError(err error) string {
	if err == nil {
		return ""
	}

	if err == errNothingToAnalyze {
		return err.Error()
	}

	if ierr, ok := err.(*errorutils.InternalError); ok {
		r.Log.Warnf("Internal error: %s", ierr.PrivateDesc)
		return ierr.PublicDesc
	}

	if berr, ok := err.(*errorutils.BadInputError); ok {
		return escapeErrorText(berr.PublicDesc, buildSecrets())
	}

	return internalError
}

func (r Repo) submitResult(ctx *RepoContext, res *repoResult, err error) {
	err = r.transformError(err)
	status := r.errorToStatus(err)
	publicErrorText := r.buildPublicError(err)

	if err == nil {
		r.Log.Infof("Succeeded repo analysis, timings: %v", res.timings)
	} else {
		r.Log.Errorf("Failed repo analysis: %s, timings: %v", err, res.timings)
	}

	if res.prepareLog != nil {
		for _, sg := range res.prepareLog.Groups {
			for _, s := range sg.Steps {
				if s.Error != "" {
					text := fmt.Sprintf("%s error: %s", s.Description, s.Error)
					text = escapeErrorText(text, buildSecrets())
					res.publicWarn(sg.Name, text)
				}
			}
		}
	}

	resJSON := &resultJSON{
		Version: 1,
		WorkerRes: workerRes{
			Timings:  res.timings,
			Warnings: res.warnings,
			Error:    publicErrorText,
		},
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
		r.Log.Warnf("Failed to marshal json: %s", err)
	} else {
		r.Log.Infof("Save repo analysis status: status=%s, result_json=%s", status, string(jsonBytes))
	}

	updateCtx := context.Background() // no timeout for state and status saving: it must be durable
	if err = r.State.UpdateState(updateCtx, ctx.Repo.Owner, ctx.Repo.Name, ctx.AnalysisGUID, s); err != nil {
		r.Log.Warnf("Can't set analysis %s status to '%v': %s", ctx.AnalysisGUID, s, err)
	}
}
