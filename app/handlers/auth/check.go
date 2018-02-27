package auth

import (
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/manager"
)

func checkAuth(ctx context.C) error {
	u, err := user.GetCurrent(&ctx)
	if err != nil {
		return err
	}

	ga, err := user.GetGithubAuth(&ctx)
	if err != nil {
		return err
	}

	au := returntypes.AuthorizedUser{
		ID:          u.ID,
		Name:        u.Name,
		AvatarURL:   u.AvatarURL,
		GithubLogin: ga.Login,
	}
	ctx.ReturnJSON(map[string]interface{}{
		"user": au,
	})
	return nil
}

func init() {
	manager.Register("/v1/auth/check", checkAuth)
}
