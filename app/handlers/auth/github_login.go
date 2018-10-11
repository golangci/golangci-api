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

const (
	publicCallbackURL  = "/v1/auth/github/callback/public"
	privateCallbackURL = "/v1/auth/github/callback/private"
)

func getWebRoot() string {
	return os.Getenv("WEB_ROOT")
}

func getAfterLoginURL() string {
	return getWebRoot() + "/repos/github?after=login"
}

func githubLogin(ctx context.C) error {
	if u, err := user.GetCurrent(&ctx); err == nil {
		if ctx.R.URL.Query().Get("relogin") != "1" {
			ctx.L.Warnf("User is already authorized: %v", u)
			ctx.RedirectTemp(getAfterLoginURL())
			return nil
		}

		auth, err := user.GetAuth(&ctx)
		if err != nil {
			ctx.L.Warnf("Can't get current auth: %s", err)
			ctx.RedirectTemp(getAfterLoginURL())
			return nil
		}

		if auth.PrivateAccessToken != "" {
			ctx.L.Infof("Github private oauth relogin")
			return githubPrivateLogin(ctx)
		}

		ctx.L.Infof("Github public oauth relogin")
		// continue authorization
	}

	a := oauth.GetPublicReposAuthorizer(publicCallbackURL)
	return a.RedirectToProvider(&ctx)
}

func githubOAuthCallback(ctx context.C) error {
	a := oauth.GetPublicReposAuthorizer(publicCallbackURL)
	gu, err := a.HandleProviderCallback(&ctx)
	if err != nil {
		return fmt.Errorf("can't complete public github oauth: %s", err)
	}

	ctx.L.Infof("Github public oauth completed: %+v", gu)
	if err = user.LoginGithub(&ctx, gu); err != nil {
		return err
	}

	ctx.RedirectTemp(getAfterLoginURL())
	return nil
}

func githubPrivateLogin(ctx context.C) error {
	a := oauth.GetPrivateReposAuthorizer(privateCallbackURL)
	return a.RedirectToProvider(&ctx)
}

func githubPrivateOAuthCallback(ctx context.C) error {
	a := oauth.GetPrivateReposAuthorizer(privateCallbackURL)
	gu, err := a.HandleProviderCallback(&ctx)
	if err != nil {
		return fmt.Errorf("can't complete private github oauth: %s", err)
	}

	ga, err := user.GetAuth(&ctx)
	if err != nil {
		return err
	}

	ga.PrivateAccessToken = gu.AccessToken
	if err := ga.Update(db.Get(&ctx), models.AuthDBSchema.PrivateAccessToken); err != nil {
		return fmt.Errorf("can't save private access token: %s", err)
	}

	ctx.RedirectTemp(getWebRoot() + "/repos/github?refresh=1&after=private_login")
	return nil
}

func init() {
	handlers.Register("/v1/auth/github", githubLogin)
	handlers.Register("/v1/auth/github/private", githubPrivateLogin)
	handlers.Register(publicCallbackURL, githubOAuthCallback)
	handlers.Register(privateCallbackURL, githubPrivateOAuthCallback)
}
