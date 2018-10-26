package implementations

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
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

func (p StableProvider) SetBaseURL(s string) error {
	return p.underlying.SetBaseURL(s)
}

func (p StableProvider) retry(f func() error) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = p.totalTimeout

	bmr := backoff.WithMaxRetries(b, uint64(p.maxRetries))
	if err := backoff.Retry(f, bmr); err != nil {
		return err
	}

	return nil
}

func (p StableProvider) GetRepoByName(ctx context.Context, owner, repo string) (retRepo *provider.Repo, err error) {
	_ = p.retry(func() error {
		retRepo, err = p.underlying.GetRepoByName(ctx, owner, repo)
		return err
	})
	return
}

func (p StableProvider) GetOrgByName(ctx context.Context, org string) (retOrg *provider.Org, err error) {
	_ = p.retry(func() error {
		retOrg, err = p.underlying.GetOrgByName(ctx, org)
		return err
	})
	return
}

func (p StableProvider) CreateRepoHook(ctx context.Context, owner, repo string,
	hook *provider.HookConfig) (*provider.Hook, error) {

	return p.underlying.CreateRepoHook(ctx, owner, repo, hook)
}

func (p StableProvider) ListRepoHooks(ctx context.Context, owner, repo string) (ret []provider.Hook, err error) {
	_ = p.retry(func() error {
		ret, err = p.underlying.ListRepoHooks(ctx, owner, repo)
		return err
	})
	return
}

func (p StableProvider) GetBranch(ctx context.Context, owner, repo, branch string) (ret *provider.Branch, err error) {
	_ = p.retry(func() error {
		ret, err = p.underlying.GetBranch(ctx, owner, repo, branch)
		return err
	})
	return
}

func (p StableProvider) DeleteRepoHook(ctx context.Context, owner, repo string, hookID int) error {
	return p.retry(func() error {
		return p.underlying.DeleteRepoHook(ctx, owner, repo, hookID)
	})
}

func (p StableProvider) ListRepos(ctx context.Context, cfg *provider.ListReposConfig) (ret []provider.Repo, err error) {
	_ = p.retry(func() error {
		ret, err = p.underlying.ListRepos(ctx, cfg)
		return err
	})
	return
}

func (p StableProvider) ListOrgs(ctx context.Context, cfg *provider.ListOrgsConfig) (ret []provider.Org, err error) {
	_ = p.retry(func() error {
		ret, err = p.underlying.ListOrgs(ctx, cfg)
		return err
	})
	return
}

func (p StableProvider) SetCommitStatus(ctx context.Context, owner, repo, ref string, status *provider.CommitStatus) error {
	return p.retry(func() error {
		return p.underlying.SetCommitStatus(ctx, owner, repo, ref, status)
	})
}

func (p StableProvider) GetPullRequest(ctx context.Context, owner, repo string, number int) (ret *provider.PullRequest, err error) {
	_ = p.retry(func() error {
		ret, err = p.underlying.GetPullRequest(ctx, owner, repo, number)
		return err
	})
	return
}
