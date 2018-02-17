package auth

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-api/app/internal/auth/oauth"
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/manager"
)

func getWebRoot() string {
	return os.Getenv("WEB_ROOT")
}

func githubLogin(ctx context.C) error {
	if u, err := user.GetCurrent(&ctx); err == nil {
		ctx.L.Warnf("User is already authorized: %v", u)
		ctx.RedirectTemp(getWebRoot())
		return nil
	}

	oauth.BeginAuthHandler(ctx.W, ctx.R)
	return nil
}

func githubOAuthCallback(ctx context.C) error {
	gu, err := oauth.CompleteUserAuth(ctx.W, ctx.R)
	if err != nil {
		return fmt.Errorf("can't complete github oauth: %s", err)
	}

	ctx.L.Infof("Github oauth completed: %+v", gu)
	if err = user.LoginGithub(&ctx, gu); err != nil {
		return err
	}

	ctx.RedirectTemp(getWebRoot())
	return nil
}

func init() {
	manager.Register("/v1/auth/github", githubLogin)
	manager.Register("/v1/auth/github/callback", githubOAuthCallback)
}
