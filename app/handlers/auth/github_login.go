package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/auth/oauth"
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golib/server/context"
)

const callbackURL = "/v1/auth/github/callback"

func getWebRoot() string {
	return os.Getenv("WEB_ROOT")
}

func getAfterLoginURL() string {
	return getWebRoot() + "/repos/github?after=login"
}

func githubLogin(ctx context.C) error {
	if u, err := user.GetCurrent(&ctx); err == nil {
		ctx.L.Warnf("User is already authorized: %v", u)
		ctx.RedirectTemp(getAfterLoginURL())
		return nil
	}

	a := oauth.GetPublicReposGithubAuthorizer(callbackURL)
	return a.RedirectToProvider(&ctx)
}

func githubOAuthCallback(ctx context.C) error {
	a := oauth.GetPublicReposGithubAuthorizer(callbackURL)
	gu, err := a.HandleProviderCallback(&ctx)
	if err != nil {
		return fmt.Errorf("can't complete github oauth: %s", err)
	}

	// Normalize data: it's important for user with github login in different case
	gu.NickName = strings.ToLower(gu.NickName)
	gu.Email = strings.ToLower(gu.Email)

	ctx.L.Infof("Github oauth completed: %+v", gu)
	if err = user.LoginGithub(&ctx, gu); err != nil {
		return err
	}

	ctx.RedirectTemp(getAfterLoginURL())
	return nil
}

func init() {
	handlers.Register("/v1/auth/github", githubLogin)
	handlers.Register(callbackURL, githubOAuthCallback)
}
