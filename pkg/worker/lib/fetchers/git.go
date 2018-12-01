package fetchers

import (
	"context"
	"strings"

	"github.com/golangci/golangci-api/pkg/worker/analytics"

	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/pkg/errors"
)

var ErrNoBranchOrRepo = errors.New("repo or branch not found")

type Git struct{}

func NewGit() *Git {
	return &Git{}
}

func (gf Git) Fetch(ctx context.Context, repo *Repo, exec executors.Executor) error {
	args := []string{"clone", "-q", "--depth", "1", "--branch",
		repo.Ref, repo.CloneURL, "."}
	if out, err := exec.Run(ctx, "git", args...); err != nil {
		noBranchOrRepo := strings.Contains(err.Error(), "could not read Username for") ||
			strings.Contains(err.Error(), "Could not find remote branch")
		if noBranchOrRepo {
			return errors.Wrap(ErrNoBranchOrRepo, err.Error())
		}

		return errors.Wrapf(err, "can't run git cmd %v: %s", args, out)
	}

	// some repos have deps in submodules, e.g. https://github.com/orbs-network/orbs-network-go
	if out, err := exec.Run(ctx, "git", "submodule", "init"); err != nil {
		analytics.Log(ctx).Warnf("Failed to init git submodule: %s, %s", err, out)
		return nil
	}
	if out, err := exec.Run(ctx, "git", "submodule", "update", "--init", "--recursive"); err != nil {
		analytics.Log(ctx).Warnf("Failed to update git submodule: %s, %s", err, out)
		return nil
	}

	return nil
}
