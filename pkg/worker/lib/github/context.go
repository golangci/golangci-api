package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/github"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type Repo struct {
	Owner, Name string
}

func (r Repo) FullName() string {
	return fmt.Sprintf("%s/%s", r.Owner, r.Name)
}

type Context struct {
	Repo              Repo
	GithubAccessToken string
	PullRequestNumber int
}

func (c Context) GetClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: c.GithubAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func (c Context) GetCloneURL(repo *gh.Repository) string {
	if repo.GetPrivate() {
		return fmt.Sprintf("https://%s@github.com/%s/%s.git",
			c.GithubAccessToken, // it's already the private token
			c.Repo.Owner, c.Repo.Name)
	}

	return repo.GetCloneURL()
}

var FakeContext = Context{
	Repo: Repo{
		Owner: "owner",
		Name:  "name",
	},
	GithubAccessToken: "access_token",
	PullRequestNumber: 1,
}
