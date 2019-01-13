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

	return c.wrapConsuming(ctx, log, func() error {
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
}
