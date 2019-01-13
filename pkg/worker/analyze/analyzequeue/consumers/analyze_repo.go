package consumers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/pkg/errors"
)

type AnalyzeRepo struct {
	baseConsumer

	rpf *processors.RepoProcessorFactory
	log logutil.Log
	cfg config.Config
}

func NewAnalyzeRepo(rpf *processors.RepoProcessorFactory, log logutil.Log, cfg config.Config) *AnalyzeRepo {
	return &AnalyzeRepo{
		baseConsumer: baseConsumer{
			eventName: analytics.EventRepoAnalyzed,
		},
		rpf: rpf,
		log: log,
		cfg: cfg,
	}
}

func (c AnalyzeRepo) Consume(ctx context.Context, repoName, analysisGUID, branch, privateAccessToken string) error {
	lctx := logutil.Context{
		"branch":       branch,
		"analysisGUID": analysisGUID,
		"provider":     "github",
		"repoName":     repoName,
		"analysisType": "repo",
	}
	log := logutil.WrapLogWithContext(c.log, lctx)

	if c.cfg.GetBool("DISABLE_REPO_ANALYSIS", false) {
		log.Warnf("Repo analysis is disabled, return error to try it later")
		return errors.New("repo analysis is disabled")
	}

	return c.wrapConsuming(ctx, log, func() error {
		var cancel context.CancelFunc
		// If you change timeout value don't forget to change it
		// in golangci-api stale analyzes checker
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		return c.analyzeRepo(ctx, log, repoName, analysisGUID, branch, privateAccessToken)
	})
}

func (c AnalyzeRepo) analyzeRepo(ctx context.Context, log logutil.Log,
	repoName, analysisGUID, branch, privateAccessToken string) error {

	parts := strings.Split(repoName, "/")
	repo := &github.Repo{
		Owner: parts[0],
		Name:  parts[1],
	}
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo name %s", repoName)
	}

	repoCtx := &processors.RepoContext{
		Ctx:                ctx,
		AnalysisGUID:       analysisGUID,
		Branch:             branch,
		Repo:               repo,
		PrivateAccessToken: privateAccessToken,
		Log:                log,
	}
	p, cleanup, err := c.rpf.BuildProcessor(repoCtx)
	if err != nil {
		return errors.Wrap(err, "failed to build repo processor")
	}
	defer cleanup()

	return p.Process(repoCtx)
}
