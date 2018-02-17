package github

import (
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var GetClient = getClient

func getClient(ctx *context.C) (*gh.Client, error) {
	ga, err := user.GetGithubAuth(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get current github auth")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: ga.AccessToken},
	)
	tc := oauth2.NewClient(ctx.Ctx, ts)
	client := gh.NewClient(tc)
	return client, nil
}
