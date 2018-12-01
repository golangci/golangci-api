package processors

import (
	"context"
	"fmt"
	"os"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/pkg/errors"
)

func makeExecutor(ctx context.Context, repo *github.Repo, forPull bool, log logutil.Log, ec *experiments.Checker) (executors.Executor, error) {
	if log == nil { // TODO: remove
		log = logutil.NewStderrLog("executor")
		log.SetLevel(logutil.LogLevelInfo)
	}
	if ec == nil { // TODO: remove
		cfg := config.NewEnvConfig(log)
		ec = experiments.NewChecker(cfg, log)
	}

	if ec.IsActiveForAnalysis("use_container_executor", repo, forPull) {
		ce, err := executors.NewContainer(log)
		if err != nil {
			return nil, errors.Wrap(err, "can't build container executor")
		}

		if err = ce.Setup(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to setup container executor")
		}
		return ce.WithWorkDir("/goapp"), nil
	}

	s := executors.NewRemoteShell(
		os.Getenv("REMOTE_SHELL_USER"),
		os.Getenv("REMOTE_SHELL_HOST"),
		os.Getenv("REMOTE_SHELL_KEY_FILE_PATH"),
	)
	if err := s.SetupTempWorkDir(ctx); err != nil {
		return nil, fmt.Errorf("can't setup temp work dir: %s", err)
	}

	return s, nil
}
