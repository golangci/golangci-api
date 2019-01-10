package implementations

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
)

// Check the struct is implementing the Provider interface.
var _ provider.Provider = &StableProvider{}

type StableProvider struct {
	underlying   provider.Provider
	totalTimeout time.Duration
	maxRetries   int
}

func NewStableProvider(underlying provider.Provider, totalTimeout time.Duration, maxRetries int) *StableProvider {
	return &StableProvider{
		underlying:   underlying,
		totalTimeout: totalTimeout,
		maxRetries:   maxRetries,
	}
}

func (p StableProvider) Name() string {
	return p.underlying.Name()
}

func (p StableProvider) LinkToPullRequest(repo *models.Repo, num int) string {
	return p.underlying.LinkToPullRequest(repo, num)
}

func (p StableProvider) SetBaseURL(s string) error {
	return p.underlying.SetBaseURL(s)
}

func (p StableProvider) retryErr(f func() error) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = p.totalTimeout

	bmr := backoff.WithMaxRetries(b, uint64(p.maxRetries))
	if err := backoff.Retry(f, bmr); err != nil {
		return err
	}

	return nil
}

func (p StableProvider) retryVoid(f func()) {
	_ = p.retryErr(func() error {
		f()
		return nil
	})
}

func (p StableProvider) GetRepoByName(ctx context.Context, owner, repo string) (retRepo *provider.Repo, err error) {
	p.retryVoid(func() {
		retRepo, err = p.underlying.GetRepoByName(ctx, owner, repo)
	})
	return
}

func (p StableProvider) GetOrgMembershipByName(ctx context.Context, org string) (retOrg *provider.OrgMembership, err error) {
	p.retryVoid(func() {
		retOrg, err = p.underlying.GetOrgMembershipByName(ctx, org)
	})
	return
}

func (p StableProvider) CreateRepoHook(ctx context.Context, owner, repo string,
	hook *provider.HookConfig) (ret *provider.Hook, err error) {

	p.retryVoid(func() {
		ret, err = p.underlying.CreateRepoHook(ctx, owner, repo, hook)
	})
	return
}

func (p StableProvider) ListRepoHooks(ctx context.Context, owner, repo string) (ret []provider.Hook, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.ListRepoHooks(ctx, owner, repo)
	})
	return
}

func (p StableProvider) GetBranch(ctx context.Context, owner, repo, branch string) (ret *provider.Branch, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.GetBranch(ctx, owner, repo, branch)
	})
	return
}

func (p StableProvider) DeleteRepoHook(ctx context.Context, owner, repo string, hookID int) error {
	return p.retryErr(func() error {
		return p.underlying.DeleteRepoHook(ctx, owner, repo, hookID)
	})
}

func (p StableProvider) ListRepos(ctx context.Context, cfg *provider.ListReposConfig) (ret []provider.Repo, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.ListRepos(ctx, cfg)
	})
	return
}

func (p StableProvider) ListOrgMemberships(ctx context.Context, cfg *provider.ListOrgsConfig) (ret []provider.OrgMembership, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.ListOrgMemberships(ctx, cfg)
	})
	return
}

func (p StableProvider) ListPullRequestCommits(ctx context.Context, owner, repo string, number int) (ret []*provider.Commit, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.ListPullRequestCommits(ctx, owner, repo, number)
	})
	return
}

func (p StableProvider) SetCommitStatus(ctx context.Context, owner, repo, ref string, status *provider.CommitStatus) error {
	return p.retryErr(func() error {
		return p.underlying.SetCommitStatus(ctx, owner, repo, ref, status)
	})
}

func (p StableProvider) GetPullRequest(ctx context.Context, owner, repo string, number int) (ret *provider.PullRequest, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.GetPullRequest(ctx, owner, repo, number)
	})
	return
}

func (p StableProvider) ParsePullRequestEvent(ctx context.Context, payload []byte) (ret *provider.PullRequestEvent, err error) {
	p.retryVoid(func() {
		ret, err = p.underlying.ParsePullRequestEvent(ctx, payload)
	})
	return
}
