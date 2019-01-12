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

func (w *Go) Setup(ctx context.Context, buildLog *result.Log, privateAccessToken string, repo *fetchers.Repo, projectPathParts ...string) (executors.Executor, error) {
	groupErr := buildLog.RunNewGroup("clone repo", func(sg *result.StepGroup) error {
		return w.repoFetcher.Fetch(ctx, sg, repo, w.exec)
	})
	if groupErr != nil {
		return nil, groupErr
	}

	exec := w.exec.
		WithEnv("REPO", path.Join(projectPathParts...)).
		WithEnv("FORMAT_JSON", "1")
	if privateAccessToken != "" {
		exec = exec.WithEnv("PRIVATE_ACCESS_TOKEN", privateAccessToken)
	}

	var envbuildResult result.Result

	groupErr = buildLog.RunNewGroup("run goenvbuild", func(sg *result.StepGroup) error {
		sg.AddStepCmd("goenvbuild")
		out, err := exec.Run(ctx, "goenvbuild")
		if err != nil {
			return errors.Wrap(err, "goenvbuild failed")
		}

		if err = json.Unmarshal([]byte(out), &envbuildResult); err != nil {
			return errors.Wrap(err, "failed to unmarshal goenvbuild result json")
		}

		return nil
	})
	if groupErr != nil {
		return nil, groupErr
	}

	// remove last group: it was needed only if error occurred
	buildLog.Groups = buildLog.Groups[:len(buildLog.Groups)-1]

	if envbuildResult.Log != nil && envbuildResult.Log.Groups != nil {
		buildLog.Groups = append(buildLog.Groups, envbuildResult.Log.Groups...)
	}

	if envbuildResult.Error != "" {
		return nil, fmt.Errorf("goenvbuild internal error: %s", envbuildResult.Error)
	}

	retExec := w.exec.WithWorkDir(envbuildResult.WorkDir)
	for k, v := range envbuildResult.Environment {
		retExec = retExec.WithEnv(k, v)
	}

	return retExec, nil
}
