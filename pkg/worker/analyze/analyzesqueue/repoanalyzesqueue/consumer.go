package repoanalyzesqueue

import (
	"context"

	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	analyzesConsumers "github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/consumers"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue"
	redsync "gopkg.in/redsync.v1"
)

type Consumer struct {
	subConsumer *analyzesConsumers.AnalyzeRepo
}

func NewConsumer(subConsumer *analyzesConsumers.AnalyzeRepo) *Consumer {
	return &Consumer{
		subConsumer: subConsumer,
	}
}

func (c Consumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return analyzesqueue.RegisterConsumer(c.consumeMessage, runQueueID, m, df)
}

func (c Consumer) consumeMessage(ctx context.Context, m *runMessage) error {
	return c.subConsumer.Consume(ctx, m.RepoName, m.AnalysisGUID, m.Branch, m.PrivateAccessToken)
}
