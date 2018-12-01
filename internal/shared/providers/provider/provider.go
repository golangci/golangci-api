package provider

import (
	"context"

	"github.com/golangci/golangci-api/pkg/api/models"
)

type Provider interface {
	Name() string

	LinkToPullRequest(repo *models.Repo, num int) string

	SetBaseURL(url string) error

	GetBranch(ctx context.Context, owner, repo, branch string) (*Branch, error)
	GetRepoByName(ctx context.Context, owner, repo string) (*Repo, error)
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error)

	GetOrgByName(ctx context.Context, org string) (*Org, error)
	GetOrgByID(ctx context.Context, orgID int) (*Org, error)

	ListRepoHooks(ctx context.Context, owner, repo string) ([]Hook, error)
	CreateRepoHook(ctx context.Context, owner, repo string, hook *HookConfig) (*Hook, error)
	DeleteRepoHook(ctx context.Context, owner, repo string, hookID int) error

	ListRepos(ctx context.Context, cfg *ListReposConfig) ([]Repo, error)
	ListOrgs(ctx context.Context, cfg *ListOrgsConfig) ([]Org, error)

	SetCommitStatus(ctx context.Context, owner, repo, ref string, status *CommitStatus) error
}
