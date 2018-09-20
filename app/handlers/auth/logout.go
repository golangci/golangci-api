package auth

import (
	"fmt"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/todo/auth/sess"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/repos"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/helpers"
	"github.com/golangci/golib/server/handlers/herrors"
)

func logout(ctx context.C) error {
	if err := sess.Remove(ctx.R, ctx.W); err != nil {
		return herrors.New(err, "can't remove session")
	}

	ctx.RedirectTemp(getWebRoot() + "?after=logout")
	return nil
}

func unlinkGithub(ctx context.C) error {
	finishTx, err := db.BeginTx(&ctx)
	if err != nil {
		return fmt.Errorf("can't start tx: %s", err)
	}
	defer finishTx(&err)

	if err := user.UnlinkGithub(&ctx); err != nil {
		return herrors.New(err, "can't unlink github")
	}

	if err := repos.DeactivateAll(&ctx); err != nil {
		return herrors.New(err, "can't unlink github")
	}

	return nil
}

func init() {
	handlers.Register("/v1/auth/logout", logout)
	handlers.Register("/v1/auth/github/unlink", helpers.OnlyPUT(unlinkGithub))
}
