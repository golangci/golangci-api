package implementations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/providers/provider"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

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

func parseGithubRepository(r *github.Repository) *provider.Repo {
	return &provider.Repo{
		ID:            r.GetID(),
		Name:          r.GetFullName(),
		IsAdmin:       r.GetPermissions()["admin"],
		IsPrivate:     r.GetPrivate(),
		DefaultBranch: r.GetDefaultBranch(),
	}
}

func (p Github) GetRepoByName(ctx context.Context, owner, repo string) (*provider.Repo, error) {
	r, _, err := p.client(ctx).Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, p.unwrapError(err)
	}

	return parseGithubRepository(r), nil
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
			ret = append(ret, *parseGithubRepository(r))
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
