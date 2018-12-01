package consumers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/task"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

var ProcessorFactory = processors.NewGithubFactory()

type AnalyzePR struct {
	baseConsumer
}

func NewAnalyzePR() *AnalyzePR {
	return &AnalyzePR{
		baseConsumer: baseConsumer{
			eventName:           analytics.EventPRChecked,
			needSendToAnalytics: true,
		},
	}
}

func (c AnalyzePR) Consume(ctx context.Context, repoOwner, repoName, githubAccessToken string,
	pullRequestNumber int, APIRequestID string, userID uint, analysisGUID string) error {

	t := &task.PRAnalysis{
		Context: github.Context{
			Repo: github.Repo{
				Owner: repoOwner,
				Name:  repoName,
			},
			GithubAccessToken: githubAccessToken,
			PullRequestNumber: pullRequestNumber,
		},
		APIRequestID: APIRequestID,
		UserID:       userID,
		AnalysisGUID: analysisGUID,
	}

	ctx = c.prepareContext(ctx, map[string]interface{}{
		"repoName":     fmt.Sprintf("%s/%s", repoOwner, repoName),
		"provider":     "github",
		"prNumber":     pullRequestNumber,
		"userIDString": strconv.Itoa(int(userID)),
		"analysisGUID": analysisGUID,
	})

	return c.wrapConsuming(ctx, func() error {
		var cancel context.CancelFunc
		// If you change timeout value don't forget to change it
		// in golangci-api stale analyzes checker
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		p, err := ProcessorFactory.BuildProcessor(ctx, t)
		if err != nil {
			return fmt.Errorf("can't build processor for task %+v: %s", t, err)
		}

		if err = p.Process(ctx); err != nil {
			return fmt.Errorf("can't process pr analysis of %+v: %s", t, err)
		}

		return nil
	})
}
