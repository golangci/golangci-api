package consumers

import (
	"context"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

type AnalyzePR struct {
	baseConsumer

	pf  processors.PullProcessorFactory
	log logutil.Log
}

func NewAnalyzePR(pf processors.PullProcessorFactory, log logutil.Log) *AnalyzePR {
	return &AnalyzePR{
		baseConsumer: baseConsumer{
			eventName: analytics.EventPRChecked,
		},
		pf:  pf,
		log: log,
	}
}

func (c AnalyzePR) Consume(ctx context.Context, repoOwner, repoName, githubAccessToken string,
	pullRequestNumber int, APIRequestID string, userID uint, analysisGUID string) error {

	repo := github.Repo{
		Owner: repoOwner,
		Name:  repoName,
	}
	lctx := logutil.Context{
		"analysisGUID": analysisGUID,
		"provider":     "github",
		"repoName":     repo.FullName(),
		"analysisType": "pull",
		"prNumber":     pullRequestNumber,
		"userIDString": strconv.Itoa(int(userID)),
	}
	ctx = c.prepareContext(ctx, lctx)
	log := logutil.WrapLogWithContext(c.log, lctx)

	startedAt := time.Now()
	finalErr := c.wrapConsuming(log, func() error {
		var cancel context.CancelFunc
		// If you change timeout value don't forget to change it
		// in golangci-api stale analyzes checker
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		pullCtx := &processors.PullContext{
			Ctx:          ctx,
			UserID:       int(userID),
			AnalysisGUID: analysisGUID,
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

	c.sendAnalytics(ctx, time.Since(startedAt), finalErr)

	if !isRecoverableError(finalErr) {
		// error was already logged, but don't retry it: just delete from queue as processed
		return nil
	}

	return finalErr
}

func (c AnalyzePR) sendAnalytics(ctx context.Context, duration time.Duration, err error) {
	props := map[string]interface{}{
		"durationSeconds": int(duration / time.Second),
	}
	if err == nil {
		props["status"] = statusOk
	} else {
		props["status"] = statusFail
		props["error"] = err.Error()
	}
	analytics.SaveEventProps(ctx, c.eventName, props)

	tracker := analytics.GetTracker(ctx)
	tracker.Track(ctx, c.eventName)
}
