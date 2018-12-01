package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/pkg/errors"
)

type Go struct {
	exec        executors.Executor
	log         logutil.Log
	repoFetcher fetchers.Fetcher
}

var _ Installer = &Go{}

func NewGo(exec executors.Executor, log logutil.Log, repoFetcher fetchers.Fetcher) *Go {
	return &Go{
		exec:        exec,
		log:         log,
		repoFetcher: repoFetcher,
	}
}

func (w *Go) Setup(ctx context.Context, repo *fetchers.Repo, projectPathParts ...string) (executors.Executor, *result.Log, error) {
	if err := w.repoFetcher.Fetch(ctx, repo, w.exec); err != nil {
		return nil, nil, errors.Wrap(err, "failed to fetch repo")
	}

	exec := w.exec.WithEnv("REPO", path.Join(projectPathParts...)).WithEnv("FORMAT_JSON", "1")
	out, err := exec.Run(ctx, "goenvbuild")
	if err != nil {
		return nil, nil, errors.Wrap(err, "goenvbuild failed")
	}

	var envbuildResult result.Result
	if err = json.Unmarshal([]byte(out), &envbuildResult); err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal goenvbuild result json")
	}

	w.log.Infof("Got envbuild result %s", out)
	if envbuildResult.Error != "" {
		return nil, nil, fmt.Errorf("goenvbuild internal error: %s", envbuildResult.Error)
	}

	retExec := w.exec.WithWorkDir(envbuildResult.WorkDir)
	for k, v := range envbuildResult.Environment {
		retExec = retExec.WithEnv(k, v)
	}

	return retExec, envbuildResult.Log, nil
}
