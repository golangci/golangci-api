package implementations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Check the struct is implementing the Provider interface.
var _ provider.Provider = &Github{}

const GithubProviderName = "github.com"

type Github struct {
	auth    *models.Auth
	baseURL *url.URL
	log     logutil.Log
}

func NewGithub(auth *models.Auth, log logutil.Log) *Github {
	return &Github{
		auth: auth,
		log:  log,
	}
}

func (p Github) Name() string {
	return GithubProviderName
}

func (p Github) LinkToPullRequest(repo *models.Repo, num int) string {
	return fmt.Sprintf("https://github.com/%s/pull/%d", repo.DisplayName, num)
}

func (p *Github) SetBaseURL(s string) error {
	baseURL, err := url.Parse(s)
	if err != nil {
		return errors.Wrap(err, "failed to parse url")
	}

	p.baseURL = baseURL
	return nil
}

func (p Github) client(ctx context.Context) *github.Client {
	at := p.auth.AccessToken
	if p.auth.PrivateAccessToken != "" {
		at = p.auth.PrivateAccessToken
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: at,
		},
	)
	tc := oauth2.NewClient(ctx, ts)
	c := github.NewClient(tc)
	if p.baseURL != nil {
		c.BaseURL = p.baseURL
	}

	return c
}

func (p Github) unwrapError(err error) error {
	if er, ok := err.(*github.ErrorResponse); ok {
		if er.Response.StatusCode == http.StatusNotFound {
			return provider.ErrNotFound
		}
		if er.Response.StatusCode == http.StatusUnauthorized {
			return provider.ErrUnauthorized
		}
	}

	return err
}

func parseGithubRepository(r *github.Repository, root bool) *provider.Repo {
	var source *provider.Repo
	if root && r.GetSource() != nil { // repository is a fork, select source
		source = parseGithubRepository(r.GetSource(), false)
	}

	var orgName string
	if r.GetOrganization() != nil {
		orgName = r.GetOrganization().GetLogin()
	}

	return &provider.Repo{
		ID:              r.GetID(),
		Name:            r.GetFullName(),
		IsAdmin:         r.GetPermissions()["admin"],
		IsPrivate:       r.GetPrivate(),
		DefaultBranch:   r.GetDefaultBranch(),
		Source:          source,
		StargazersCount: r.GetStargazersCount(),
		Language:        r.GetLanguage(),
		Organization:    orgName,
	}
}

func parseGithubOrganization(m *github.Membership) *provider.Org {
	return &provider.Org{
		ID:      m.GetOrganization().GetID(),
		Name:    m.GetOrganization().GetLogin(),
		IsAdmin: m.GetRole() == "admin" && m.GetState() == "active",
	}
}

func (p Github) GetRepoByName(ctx context.Context, owner, repo string) (*provider.Repo, error) {
	r, _, err := p.client(ctx).Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return parseGithubRepository(r, true), nil
}

func (p Github) GetOrgByName(ctx context.Context, org string) (*provider.Org, error) {
	m, _, err := p.client(ctx).Organizations.GetOrgMembership(ctx, "", org)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return parseGithubOrganization(m), nil
}

func (p Github) GetOrgByID(ctx context.Context, orgID int) (*provider.Org, error) {
	o, _, err := p.client(ctx).Organizations.GetByID(ctx, orgID)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return p.GetOrgByName(ctx, o.GetName())
}

func (p Github) parseHook(h *github.Hook) *provider.Hook {
	ctObj := h.Config["content_type"]
	ct := ""
	if ctObj != nil {
		if ctStr, ok := ctObj.(string); ok {
			ct = ctStr
		}
	}

	return &provider.Hook{
		HookConfig: provider.HookConfig{
			Name:        h.GetName(),
			Events:      h.Events,
			URL:         h.GetURL(),
			ContentType: ct,
		},
		ID: h.GetID(),
	}
}

func (p Github) CreateRepoHook(ctx context.Context, owner, repo string,
	hook *provider.HookConfig) (*provider.Hook, error) {

	githubHookCfg := github.Hook{
		Name:   &hook.Name,
		Events: hook.Events,
		Config: map[string]interface{}{
			"url":          hook.URL,
			"content_type": hook.ContentType,
		},
	}
	rh, _, err := p.client(ctx).Repositories.CreateHook(ctx, owner, repo, &githubHookCfg)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return p.parseHook(rh), nil
}

func (p Github) ListRepoHooks(ctx context.Context, owner, repo string) ([]provider.Hook, error) {
	listOptions := github.ListOptions{
		PerPage: 100,
	}
	hooks, _, err := p.client(ctx).Repositories.ListHooks(ctx, owner, repo, &listOptions)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	if len(hooks) == listOptions.PerPage {
		return nil, fmt.Errorf("repo %s/%s has >%d hooks, need to support pagination",
			owner, repo, len(hooks))
	}

	var retHooks []provider.Hook
	for _, h := range hooks {
		retHooks = append(retHooks, *p.parseHook(h))
	}
	return retHooks, nil
}

func (p Github) GetBranch(ctx context.Context, owner, repo, branch string) (*provider.Branch, error) {
	grb, _, err := p.client(ctx).Repositories.GetBranch(ctx, owner, repo, branch)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return &provider.Branch{
		HeadCommitSHA: grb.GetCommit().GetSHA(),
	}, nil
}

func (p Github) DeleteRepoHook(ctx context.Context, owner, repo string, hookID int) error {
	_, err := p.client(ctx).Repositories.DeleteHook(ctx, owner, repo, hookID)
	if err != nil {
		return p.unwrapError(err)
	}

	return nil
}

func (p Github) SetCommitStatus(ctx context.Context, owner, repo, ref string, status *provider.CommitStatus) error {
	githubStatus := github.RepoStatus{
		State:       github.String(status.State),
		Description: github.String(status.Description),
		Context:     github.String(status.Context),
	}
	if status.TargetURL != "" {
		githubStatus.TargetURL = github.String(status.TargetURL)
	}

	_, _, err := p.client(ctx).Repositories.CreateStatus(ctx, owner, repo, ref, &githubStatus)
	if err != nil {
		return p.unwrapError(err)
	}

	return nil
}

func (p Github) ListRepos(ctx context.Context, cfg *provider.ListReposConfig) ([]provider.Repo, error) {
	opts := github.RepositoryListOptions{
		Visibility: cfg.Visibility,
		Sort:       cfg.Sort,
		ListOptions: github.ListOptions{
			PerPage: 100, // 100 is a max allowed value
		},
	}

	var ret []provider.Repo
	for {
		pageRepos, resp, err := p.client(ctx).Repositories.List(ctx, "", &opts)
		if err != nil {
			return nil, p.unwrapError(err)
		}

		for _, r := range pageRepos {
			ret = append(ret, *parseGithubRepository(r, true))
		}

		if resp.NextPage == 0 { // it's the last page
			break
		}

		if opts.Page == cfg.MaxPages { // TODO: fetch all, now we limit it
			p.log.Warnf("Limited repo list to %d entries (%d pages)", len(ret), cfg.MaxPages)
			break
		}

		opts.Page = resp.NextPage
	}

	return ret, nil
}

func (p Github) ListOrgs(ctx context.Context, cfg *provider.ListOrgsConfig) ([]provider.Org, error) {
	opts := github.ListOrgMembershipsOptions{
		State: cfg.MembershipState,
		ListOptions: github.ListOptions{
			PerPage: 100, // 100 is a max allowed value
		},
	}

	var ret []provider.Org
	for {
		pageMemberships, resp, err := p.client(ctx).Organizations.ListOrgMemberships(ctx, &opts)
		if err != nil {
			return nil, p.unwrapError(err)
		}

		for _, m := range pageMemberships {
			ret = append(ret, *parseGithubOrganization(m))
		}

		if resp.NextPage == 0 { // it's the last page
			break
		}

		if opts.Page == cfg.MaxPages { // TODO: fetch all, now we limit it
			p.log.Warnf("Limited repo list to %d entries (%d pages)", len(ret), cfg.MaxPages)
			break
		}

		opts.Page = resp.NextPage
	}

	return ret, nil
}

func (p Github) GetPullRequest(ctx context.Context, owner, repo string, number int) (*provider.PullRequest, error) {
	pr, _, err := p.client(ctx).PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return &provider.PullRequest{
		HeadCommitSHA: pr.GetHead().GetSHA(),
		State:         pr.GetState(),
	}, nil
}
