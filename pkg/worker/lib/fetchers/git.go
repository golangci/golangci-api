package fetchers

import (
	"context"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analytics"

	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/pkg/errors"
)

var ErrNoBranchOrRepo = errors.New("repo or branch not found")

type Git struct{}

func NewGit() *Git {
	return &Git{}
}

func (gf Git) Fetch(ctx context.Context, sg *result.StepGroup, repo *Repo, exec executors.Executor) error {
	args := []string{"clone", "-q", "--depth", "1", "--branch",
		repo.Ref, repo.CloneURL, "."}
	gitStep := sg.AddStepCmd("git", args...)

	out, err := exec.Run(ctx, "git", args...)
	gitStep.AddOutput(out)

	if err != nil {
		noBranchOrRepo := strings.Contains(out, "could not read Username for") ||
			strings.Contains(out, "Could not find remote branch")
		if noBranchOrRepo {
			return ErrNoBranchOrRepo
		}

		return errors.Wrapf(err, "can't run git cmd %v: %s", args, out)
	}

	// some repos have deps in submodules, e.g. https://github.com/orbs-network/orbs-network-go
	submoduleInitStep := sg.AddStepCmd("git", "submodule", "init")
	out, err = exec.Run(ctx, "git", "submodule", "init")
	submoduleInitStep.AddOutput(out)
	if err != nil {
		analytics.Log(ctx).Warnf("Failed to init git submodule: %s", err)
		return nil
	}

	submoduleUpdateStep := sg.AddStepCmd("git", "submodule", "update", "--init", "--recursive")
	out, err = exec.Run(ctx, "git", "submodule", "update", "--init", "--recursive")
	submoduleUpdateStep.AddOutput(out)
	if err != nil {
		analytics.Log(ctx).Warnf("Failed to update git submodule: %s", err)
		return nil
	}

	return nil
}
