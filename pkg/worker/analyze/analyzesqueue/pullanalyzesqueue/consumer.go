package pullanalyzesqueue

import (
	"context"

	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	analyzesConsumers "github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/consumers"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue"
	redsync "gopkg.in/redsync.v1"
)

type Consumer struct {
	subConsumer *analyzesConsumers.AnalyzePR
}

func NewConsumer(subConsumer *analyzesConsumers.AnalyzePR) *Consumer {
	return &Consumer{
		subConsumer: subConsumer,
	}
}

func (c Consumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return analyzesqueue.RegisterConsumer(c.consumeMessage, runQueueID, m, df)
}

func (c Consumer) consumeMessage(ctx context.Context, m *RunMessage) error {
	return c.subConsumer.Consume(ctx, m.Repo.Owner, m.Repo.Name,
		m.Repo.IsPrivate, m.GithubAccessToken,
		m.PullRequestNumber, m.APIRequestID, m.UserID, m.AnalysisGUID, m.CommitSHA)
}
