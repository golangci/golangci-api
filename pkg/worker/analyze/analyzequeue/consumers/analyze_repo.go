package consumers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/apperrors"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/pkg/errors"
)

type AnalyzeRepo struct {
	baseConsumer

	rpf        *processors.RepoProcessorFactory
	log        logutil.Log
	errTracker apperrors.Tracker
	cfg        config.Config
}

func NewAnalyzeRepo(rpf *processors.RepoProcessorFactory, log logutil.Log, errTracker apperrors.Tracker, cfg config.Config) *AnalyzeRepo {
	return &AnalyzeRepo{
		baseConsumer: baseConsumer{
			eventName: analytics.EventRepoAnalyzed,
			cfg:       cfg,
		},
		rpf:        rpf,
		log:        log,
		errTracker: errTracker,
		cfg:        cfg,
	}
}

func (c AnalyzeRepo) Consume(ctx context.Context, repoName, analysisGUID, branch, privateAccessToken, commitSHA string) error {
	lctx := logutil.Context{
		"branch":       branch,
		"analysisGUID": analysisGUID,
		"provider":     "github",
		"repoName":     repoName,
		"analysisType": "repo",
		"reportURL":    fmt.Sprintf("https://golangci.com/r/github.com/%s", repoName),
		"commitSHA":    commitSHA,
	}
	log := logutil.WrapLogWithContext(c.log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, c.errTracker)

	if c.cfg.GetBool("DISABLE_REPO_ANALYSIS", false) {
		log.Warnf("Repo analysis is disabled, return error to try it later")
		return errors.New("repo analysis is disabled")
	}

	return c.wrapConsuming(ctx, log, repoName, func() error {
		var cancel context.CancelFunc
		// If you change timeout value don't forget to change it
		// in golangci-api stale analyzes checker
		const containerStartupTime = time.Minute
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute+containerStartupTime)
		defer cancel()

		return c.analyzeRepo(ctx, log, repoName, analysisGUID, branch, privateAccessToken, commitSHA)
	})
}

func (c AnalyzeRepo) analyzeRepo(ctx context.Context, log logutil.Log,
	repoName, analysisGUID, branch, privateAccessToken, commitSHA string) error {

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
		CommitSHA:          commitSHA,
		Log:                log,
	}
	p, cleanup, err := c.rpf.BuildProcessor(repoCtx)
	if err != nil {
		return errors.Wrap(err, "failed to build repo processor")
	}
	defer cleanup()

	return p.Process(repoCtx)
}
