package auth

import (
	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/returntypes"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golib/server/context"
)

func checkAuth(ctx context.C) error {
	u, err := user.GetCurrent(&ctx)
	if err != nil {
		return err
	}

	ga, err := user.GetAuth(&ctx)
	if err != nil {
		return err
	}

	au := returntypes.AuthorizedUser{
		ID:          u.ID,
		Name:        u.Name,
		Email:       u.Email,
		AvatarURL:   u.AvatarURL,
		GithubLogin: ga.Login,
		CreatedAt:   u.CreatedAt,
	}
	ctx.ReturnJSON(map[string]interface{}{
		"user": au,
	})
	return nil
}

func init() {
	handlers.Register("/v1/auth/check", checkAuth)
}
