package processors

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/pkg/errors"
)

func makeExecutor(log logutil.Log, ec *experiments.Checker,
	repo *github.Repo, cfg config.Config, awsSess *session.Session, isPull bool) (executors.Executor, error) {

	if ec.IsActiveForAnalysis("FARGATE_EXECUTOR", repo, isPull) {
		return executors.NewFargate(log, cfg, awsSess).WithWorkDir("/goapp"), nil
	}

	ce, err := executors.NewContainer(log)
	if err != nil {
		return nil, errors.Wrap(err, "can't build container executor")
	}
	return ce.WithWorkDir("/goapp"), nil
}
