package resources

import (
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

func BuildExecutorRequirementsForRepo(ec *experiments.Checker, repo *github.Repo) *executors.Requirements {
	maxRequirements := executors.Requirements{
		CPUCount: 4,
		MemoryGB: 30,
	}
	if repo.IsPrivate {
		return &maxRequirements
	}

	if ec.IsActiveForRepo("MAX_RESOURCE_REQUIREMENTS", repo.Owner, repo.Name) {
		return &maxRequirements
	}

	return &executors.Requirements{
		CPUCount: 4,
		MemoryGB: 16,
	}
}
