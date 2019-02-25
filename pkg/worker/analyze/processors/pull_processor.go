package processors

import (
	"context"

	"github.com/golangci/golangci-api/pkg/goenvbuild/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	gh "github.com/google/go-github/github"
)

type PullProcessor interface {
	Process(ctx *PullContext) error
}

type PullContext struct {
	Ctx          context.Context
	UserID       int
	AnalysisGUID string
	ProviderCtx  *github.Context
	LogCtx       logutil.Context
	Log          logutil.Log

	pull *gh.PullRequest

	res         *analysisResult
	savedLog    logutil.Log
	buildConfig *config.Service
}

func (ctx *PullContext) repo() *github.Repo {
	return &ctx.ProviderCtx.Repo
}

func (ctx *PullContext) secrets() []string {
	return []string{ctx.ProviderCtx.GithubAccessToken, ctx.AnalysisGUID}
}
