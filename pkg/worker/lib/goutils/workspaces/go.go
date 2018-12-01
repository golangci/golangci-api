package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenvbuild/ensuredeps"
	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/repoinfo"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/goutils/environments"
	"github.com/pkg/errors"
)

type Go struct {
	gopath      string
	exec        executors.Executor
	infoFetcher repoinfo.Fetcher
}

func NewGo(exec executors.Executor, infoFetcher repoinfo.Fetcher) *Go {
	return &Go{
		exec:        exec,
		infoFetcher: infoFetcher,
	}
}

func (w *Go) Setup(ctx context.Context, repo *fetchers.Repo, projectPathParts ...string) error {
	repoInfo, err := w.infoFetcher.Fetch(ctx, repo, w.exec)
	if err != nil {
		return errors.Wrap(err, "failed to fetch repo info")
	}

	if repoInfo != nil && repoInfo.CanonicalImportPath != "" {
		newProjectPathParts := strings.Split(repoInfo.CanonicalImportPath, "/")
		analytics.Log(ctx).Infof("change canonical project path: %s -> %s", projectPathParts, newProjectPathParts)
		projectPathParts = newProjectPathParts
	}

	if _, err := w.exec.Run(ctx, "find", ".", "-delete"); err != nil {
		analytics.Log(ctx).Warnf("Failed to cleanup after repo info fetcher: %s", err)
	}

	gopath := w.exec.WorkDir()
	wdParts := []string{gopath, "src"}
	wdParts = append(wdParts, projectPathParts...)
	wd := filepath.Join(wdParts...)
	if out, err := w.exec.Run(ctx, "mkdir", "-p", wd); err != nil {
		return fmt.Errorf("can't create project dir %q: %s, %s", wd, err, out)
	}

	goEnv := environments.NewGolang(gopath)
	goEnv.Setup(w.exec)

	w.exec = w.exec.WithWorkDir(wd) // XXX: clean gopath, but work in subdir of gopath

	w.gopath = gopath
	return nil
}

func (w Go) Executor() executors.Executor {
	return w.exec
}

func (w Go) Gopath() string {
	return w.gopath
}

func (w Go) FetchDeps(ctx context.Context, fullRepoPath string) (*ensuredeps.Result, error) {
	cleanupPath := filepath.Join("/app", "cleanup.sh")
	out, err := w.exec.Run(ctx, "bash", cleanupPath)
	if err != nil {
		return nil, fmt.Errorf("can't call /app/cleanup.sh: %s, %s", err, out)
	}

	out, err = w.exec.Run(ctx, "ensuredeps", "--repo", fullRepoPath)
	if err != nil {
		return nil, fmt.Errorf("can't ensuredeps --repo %s: %s, %s", fullRepoPath, err, out)
	}

	var res ensuredeps.Result
	if err = json.Unmarshal([]byte(out), &res); err != nil {
		return nil, fmt.Errorf("failed to parse res json: %s", err)
	}

	return &res, nil
}

func (w Go) Clean(ctx context.Context) {
	out, err := w.exec.Run(ctx, "go", "clean", "-modcache")
	if err != nil {
		analytics.Log(ctx).Warnf("Can't clean go modcache: %s, %s", err, out)
	}
}
