package repos

import (
	"net/http"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golangci-api/app/internal/repos"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
)

func changeRepo(ctx context.C) error {
	ga, err := user.GetGithubAuth(&ctx)
	if err != nil {
		return herrors.New(err, "can't get github auth")
	}

	repoOwner := ctx.URLVar("repoOwner")
	repoName := ctx.URLVar("repoName")

	var gr *models.GithubRepo
	var activate = ctx.R.Method == http.MethodPut
	switch ctx.R.Method {
	case http.MethodPut:
		gr, err = repos.ActivateRepo(&ctx, ga, repoOwner, repoName)
		if err != nil {
			return herrors.New(err, "can't activate repo")
		}
	case http.MethodDelete:
		gr, err = repos.DeactivateRepo(&ctx, repoOwner, repoName)
		if err != nil {
			return herrors.New(err, "can't deactivate repo")
		}
	default:
		return herrors.New404Errorf("unallowed method")
	}

	ri := returntypes.RepoInfo{
		Name:        gr.Name,
		IsActivated: activate,
		HookID:      gr.HookID,
	}
	ctx.ReturnJSON(map[string]returntypes.RepoInfo{
		"repo": ri,
	})
	return nil
}

func init() {
	handlers.Register("/v1/repos/{repoOwner}/{repoName}", changeRepo)
}
