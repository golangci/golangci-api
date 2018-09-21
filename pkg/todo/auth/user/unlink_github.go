package user

import (
	"fmt"

	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
)

func UnlinkGithub(ctx *context.C) error {
	ga, err := GetAuth(ctx)
	if err != nil {
		return herrors.New(err, "can't get auth")
	}

	if err = ga.Delete(db.Get(ctx).Unscoped()); err != nil {
		return fmt.Errorf("can't delete auth: %s", err)
	}

	return nil
}
