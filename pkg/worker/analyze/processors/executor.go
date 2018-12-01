package processors

import (
	"context"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/pkg/errors"
)

func makeExecutor(ctx context.Context, log logutil.Log) (executors.Executor, error) {
	if log == nil { // TODO: remove
		log = logutil.NewStderrLog("executor")
		log.SetLevel(logutil.LogLevelInfo)
	}

	ce, err := executors.NewContainer(log)
	if err != nil {
		return nil, errors.Wrap(err, "can't build container executor")
	}

	if err = ce.Setup(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup container executor")
	}
	return ce.WithWorkDir("/goapp"), nil
}
