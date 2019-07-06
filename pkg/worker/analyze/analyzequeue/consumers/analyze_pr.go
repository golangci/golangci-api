package consumers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/internal/shared/apperrors"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

type AnalyzePR struct {
	baseConsumer

	pf         processors.PullProcessorFactory
	errTracker apperrors.Tracker
	log        logutil.Log
}

func NewAnalyzePR(pf processors.PullProcessorFactory, log logutil.Log, errTracker apperrors.Tracker, cfg config.Config, ec *experiments.Checker) *AnalyzePR {
	return &AnalyzePR{
		baseConsumer: baseConsumer{
			eventName: analytics.EventPRChecked,
			cfg:       cfg,
			ec:        ec,
		},
		pf:         pf,
		errTracker: errTracker,
		log:        log,
	}
}

func (c AnalyzePR) Consume(ctx context.Context, repoOwner, repoName string,
	isPrivateRepo bool, githubAccessToken string, pullRequestNumber int,
	apiRequestID string, userID uint, analysisGUID, commitSHA string) error {

	repo := github.Repo{
		Owner:     repoOwner,
		Name:      repoName,
		IsPrivate: isPrivateRepo,
	}
	lctx := logutil.Context{
		"analysisGUID":  analysisGUID,
		"provider":      "github",
		"repoName":      repo.FullName(),
		"analysisType":  "pull",
		"prNumber":      pullRequestNumber,
		"userIDString":  strconv.Itoa(int(userID)),
		"providerURL":   fmt.Sprintf("https://github.com/%s/pull/%d", repo.FullName(), pullRequestNumber),
		"reportURL":     fmt.Sprintf("%s/r/github.com/%s/pulls/%d", c.cfg.GetString("WEB_ROOT"), repo.FullName(), pullRequestNumber),
		"isPrivateRepo": isPrivateRepo,
		"commitSHA":     commitSHA,
	}
	ctx = c.prepareContext(ctx, lctx)
	log := logutil.WrapLogWithContext(c.log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, c.errTracker)

	return c.wrapConsuming(ctx, log, repo.FullName(), func() error {
		var cancel context.CancelFunc
		// If you change timeout value don't forget to change it
		// in golangci-api stale analyzes checker
		const containerStartupTime = time.Minute
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute+containerStartupTime)
		defer cancel()

		pullCtx := &processors.PullContext{
			Ctx:          ctx,
			UserID:       int(userID),
			AnalysisGUID: analysisGUID,
			CommitSHA:    commitSHA,
			ProviderCtx: &github.Context{
				Repo:              repo,
				GithubAccessToken: githubAccessToken,
				PullRequestNumber: pullRequestNumber,
			},
			LogCtx: lctx,
			Log:    log,
		}

		p, cleanup, err := c.pf.BuildProcessor(pullCtx)
		if err != nil {
			return errors.Wrap(err, "can't build processor")
		}
		defer cleanup()

		if err = p.Process(pullCtx); err != nil {
			return errors.Wrap(err, "can't process pull analysis")
		}

		return nil
	})
}
