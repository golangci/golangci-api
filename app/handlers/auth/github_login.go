package auth

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/auth/oauth"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golangci-api/pkg/todo/db"
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

	a := oauth.GetPublicReposAuthorizer(callbackURL)
	return a.RedirectToProvider(&ctx)
}

func githubOAuthCallback(ctx context.C) error {
	if _, err := user.GetCurrent(&ctx); err == nil {
		// User is already authorized, but we checked it in githubLogin.
		// Therefore it's a private login callback.
		return githubPrivateOAuthCallback(ctx)
	}

	a := oauth.GetPublicReposAuthorizer(callbackURL)
	gu, err := a.HandleProviderCallback(&ctx)
	if err != nil {
		return fmt.Errorf("can't complete github oauth: %s", err)
	}

	ctx.L.Infof("Github oauth completed: %+v", gu)
	if err = user.LoginGithub(&ctx, gu); err != nil {
		return err
	}

	ctx.RedirectTemp(getAfterLoginURL())
	return nil
}

func githubPrivateLogin(ctx context.C) error {
	a := oauth.GetPrivateReposAuthorizer(callbackURL)
	return a.RedirectToProvider(&ctx)
}

func githubPrivateOAuthCallback(ctx context.C) error {
	a := oauth.GetPrivateReposAuthorizer(callbackURL)
	gu, err := a.HandleProviderCallback(&ctx)
	if err != nil {
		return fmt.Errorf("can't complete github oauth: %s", err)
	}

	ga, err := user.GetAuth(&ctx)
	if err != nil {
		return err
	}

	ga.PrivateAccessToken = gu.AccessToken
	if err := ga.Update(db.Get(&ctx), models.AuthDBSchema.PrivateAccessToken); err != nil {
		return fmt.Errorf("can't save access token: %s", err)
	}

	ctx.RedirectTemp(getWebRoot() + "/repos/github?refresh=1&after=private_login")
	return nil
}

func init() {
	handlers.Register("/v1/auth/github", githubLogin)
	handlers.Register("/v1/auth/github/private", githubPrivateLogin)
	handlers.Register(callbackURL, githubOAuthCallback)
}
