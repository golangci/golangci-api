package github

import (
	gocontext "context"
	"fmt"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var GetClient = getClient

func getClient(ctx *context.C) (*gh.Client, bool, error) {
	ga, err := user.GetAuth(ctx)
	if err != nil {
		return nil, false, herrors.New(err, "can't get current auth")
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
	ga, err := user.GetAuthForUser(ctx, userID)
	if err != nil {
		return nil, false, herrors.New(err, "can't get user %d auth", userID)
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

func GetClientForUserV2(ctx gocontext.Context, db *gorm.DB, userID uint) (*gh.Client, error) {
	var ga models.Auth
	err := models.NewAuthQuerySet(db).
		UserIDEq(userID).
		OrderDescByID().
		One(&ga)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get auth for user %d", userID)
	}

	accessToken := ga.AccessToken
	if ga.PrivateAccessToken != "" {
		accessToken = ga.PrivateAccessToken
	}

	if accessToken == "" {
		return nil, fmt.Errorf("access token is empty")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			AccessToken: accessToken,
		},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := gh.NewClient(tc)

	return client, nil
}
