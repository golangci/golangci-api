package github

import (
	"fmt"

	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var GetClient = getClient

func getClient(ctx *context.C) (*gh.Client, bool, error) {
	ga, err := user.GetGithubAuth(ctx)
	if err != nil {
		return nil, false, herrors.New(err, "can't get current github auth")
	}

	at := ga.AccessToken
	needPrivateRepos := ga.PrivateAccessToken != ""
	if needPrivateRepos {
		at = ga.PrivateAccessToken
	}

	if at == "" {
		return nil, false, fmt.Errorf("access token is empty")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: at,
		},
	)
	tc := oauth2.NewClient(ctx.Ctx, ts)
	client := gh.NewClient(tc)

	return client, needPrivateRepos, nil
}

func GetClientForUser(ctx *context.C, userID uint) (*gh.Client, bool, error) {
	ga, err := user.GetGithubAuthForUser(ctx, userID)
	if err != nil {
		return nil, false, herrors.New(err, "can't get user %d github auth", userID)
	}

	at := ga.AccessToken
	needPrivateRepos := ga.PrivateAccessToken != ""
	if needPrivateRepos {
		at = ga.PrivateAccessToken
	}

	if at == "" {
		return nil, false, fmt.Errorf("access token is empty")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: at,
		},
	)
	tc := oauth2.NewClient(ctx.Ctx, ts)
	client := gh.NewClient(tc)

	return client, needPrivateRepos, nil
}
