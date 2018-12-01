package processors

import (
	"context"
	"fmt"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/task"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

type Factory interface {
	BuildProcessor(ctx context.Context, t *task.PRAnalysis) (Processor, error)
}

type githubFactory struct{}

func NewGithubFactory() Factory {
	return githubFactory{}
}

func (gf githubFactory) BuildProcessor(ctx context.Context, t *task.PRAnalysis) (Processor, error) {
	p, err := newGithubGoPR(ctx, &t.Context, githubGoPRConfig{}, t.AnalysisGUID)
	if err != nil {
		if !github.IsRecoverableError(err) {
			analytics.Log(ctx).Warnf("%s: skip current task: use nop processor", err)
			return NopProcessor{}, nil
		}
		return nil, fmt.Errorf("can't make github go processor: %s", err)
	}

	return p, nil
}
