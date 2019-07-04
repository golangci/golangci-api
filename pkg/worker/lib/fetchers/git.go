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
var ErrNoCommit = errors.New("commit not found")

type Git struct{}

func NewGit() *Git {
	return &Git{}
}

func (gf Git) clone(ctx context.Context, sg *result.StepGroup, repo *Repo, exec executors.Executor) error {
	args := []string{"clone", "-q", "--branch", repo.Ref, repo.CloneURL, "."}
	gitStep := sg.AddStepCmd("git", args...)

	cloneRes, err := exec.Run(ctx, "git", args...)
	if cloneRes != nil {
		gitStep.AddOutput(cloneRes.StdOut)
		gitStep.AddOutput(cloneRes.StdErr)
	}

	if err != nil {
		var out string
		if cloneRes != nil {
			out = cloneRes.StdOut + cloneRes.StdErr
		}
		noBranchOrRepo := strings.Contains(out, "could not read Username for") ||
			strings.Contains(out, "Could not find remote branch")
		if noBranchOrRepo {
			return ErrNoBranchOrRepo
		}

		return errors.Wrapf(err, "can't run git cmd %v: %s", args, out)
	}

	return nil
}

func (gf Git) checkout(ctx context.Context, sg *result.StepGroup, repo *Repo, exec executors.Executor) error {
	if repo.CommitSHA == "" {
		// TODO: remove it, remporary
		return nil
	}

	args := []string{"checkout", "-q", repo.CommitSHA}
	gitStep := sg.AddStepCmd("git", args...)

	checkoutRes, err := exec.Run(ctx, "git", args...)
	if checkoutRes != nil {
		gitStep.AddOutput(checkoutRes.StdOut)
		gitStep.AddOutput(checkoutRes.StdErr)
	}

	if err != nil {
		var out string
		if checkoutRes != nil {
			out = checkoutRes.StdOut + checkoutRes.StdErr
		}
		if strings.Contains(out, "did not match any file") {
			return ErrNoCommit
		}

		return errors.Wrapf(err, "can't run git cmd %v: %s", args, out)
	}

	return nil
}

func (gf Git) updateSubmodules(ctx context.Context, sg *result.StepGroup, exec executors.Executor) {
	// some repos have deps in submodules, e.g. https://github.com/orbs-network/orbs-network-go
	submoduleInitStep := sg.AddStepCmd("git", "submodule", "init")
	runRes, err := exec.Run(ctx, "git", "submodule", "init")
	submoduleInitStep.AddOutput(runRes.StdOut)
	submoduleInitStep.AddOutput(runRes.StdErr)
	if err != nil {
		analytics.Log(ctx).Infof("Failed to init git submodule: %s", err)
		return
	}

	submoduleUpdateStep := sg.AddStepCmd("git", "submodule", "update", "--init", "--recursive")
	runRes, err = exec.Run(ctx, "git", "submodule", "update", "--init", "--recursive")
	submoduleUpdateStep.AddOutput(runRes.StdOut)
	submoduleUpdateStep.AddOutput(runRes.StdErr)
	if err != nil {
		analytics.Log(ctx).Infof("Failed to update git submodule: %s", err)
		return
	}
}

func (gf Git) Fetch(ctx context.Context, sg *result.StepGroup, repo *Repo, exec executors.Executor) error {
	if err := gf.clone(ctx, sg, repo, exec); err != nil {
		return err
	}

	if err := gf.checkout(ctx, sg, repo, exec); err != nil {
		return err
	}

	gf.updateSubmodules(ctx, sg, exec)
	return nil
}
