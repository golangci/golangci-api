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

	GetOrgMembershipByName(ctx context.Context, org string) (*OrgMembership, error)

	ListRepoHooks(ctx context.Context, owner, repo string) ([]Hook, error)
	CreateRepoHook(ctx context.Context, owner, repo string, hook *HookConfig) (*Hook, error)
	DeleteRepoHook(ctx context.Context, owner, repo string, hookID int) error

	ListRepos(ctx context.Context, cfg *ListReposConfig) ([]Repo, error)
	ListOrgMemberships(ctx context.Context, cfg *ListOrgsConfig) ([]OrgMembership, error)

	ListPullRequestCommits(ctx context.Context, owner, repo string, number int) ([]*Commit, error)
	SetCommitStatus(ctx context.Context, owner, repo, ref string, status *CommitStatus) error

	ParsePullRequestEvent(ctx context.Context, payload []byte) (*PullRequestEvent, error)

	AddCollaborator(ctx context.Context, owner, repo, username string) (*RepoInvitation, error)
	RemoveCollaborator(ctx context.Context, owner, repo, username string) error
	AcceptRepoInvitation(ctx context.Context, invitationID int) error
}
