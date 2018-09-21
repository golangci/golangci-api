package repos

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
)

func DeactivateRepo(ctx *context.C, owner, repo string) (*models.Repo, error) {
	gc, _, err := github.GetClient(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get github client")
	}

	var gr models.Repo
	err = models.NewRepoQuerySet(db.Get(ctx)).
		NameEq(strings.ToLower(fmt.Sprintf("%s/%s", owner, repo))).
		One(&gr)
	if err != nil {
		if err == gorm.ErrRecordNotFound { // Race condition: double deactivation request
			return &models.Repo{}, nil
		}

		return nil, fmt.Errorf("can't get repo %s/%s: %s", owner, repo, err)
	}

	_, err = gc.Repositories.DeleteHook(ctx.Ctx, owner, repo, gr.ProviderHookID)
	if err != nil {
		if er, ok := err.(*gh.ErrorResponse); ok && er.Response.StatusCode == http.StatusNotFound {
			ctx.L.Warnf("Webhook or repo for %#v was deleted by user: deactivating repo without deleting webhook",
				repo)
		} else {
			return nil, fmt.Errorf("can't delete hook %d from repo %s/%s: %s",
				gr.ProviderHookID, owner, repo, err)
		}
	}

	// It's important to delete only after successful github hook deletion to prevent
	// deletion of foreign repo
	if err = gr.Delete(db.Get(ctx)); err != nil {
		return nil, fmt.Errorf("can't delete repo: %s", err)
	}

	return &gr, nil
}

func GetWebhookURLPathForRepo(name, hookID string) string {
	return fmt.Sprintf("/v1/repos/%s/hooks/%s", name, hookID)
}

func ActivateRepo(ctx *context.C, ga *models.Auth, owner, repo string) (*models.Repo, error) {
	origRepoName := fmt.Sprintf("%s/%s", owner, repo)
	repoName := strings.ToLower(origRepoName)

	var gr models.Repo
	err := models.NewRepoQuerySet(db.Get(ctx)).NameEq(repoName).One(&gr) // TODO: match by id here
	if err == nil {
		ctx.L.Infof("user attempts to activate repo twice")
		return &gr, nil
	}

	gc, _, err := github.GetClient(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get github client")
	}

	hookID, err := utils.GenerateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("can't generate hook id: %s", err)
	}

	gr = models.Repo{
		UserID:      ga.UserID,
		Name:        repoName,
		DisplayName: origRepoName,
		HookID:      hookID,
		Provider:    "github.com",
	}

	web := "web"
	hookURL := os.Getenv("GITHUB_CALLBACK_HOST") + GetWebhookURLPathForRepo(gr.Name, gr.HookID)
	hook := gh.Hook{
		Name:   &web,
		Events: []string{"push", "pull_request"},
		Config: map[string]interface{}{
			"url":          hookURL,
			"content_type": "json",
		},
	}
	rh, _, err := gc.Repositories.CreateHook(ctx.Ctx, owner, repo, &hook)
	if err != nil {
		return nil, fmt.Errorf("can't post hook %v to github: %s", hook, err)
	}

	githubRepo, _, err := gc.Repositories.Get(ctx.Ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("can't get repo %s/%s data: %s", owner, repo, err)
	}
	gr.ProviderID = githubRepo.GetID()

	gr.ProviderHookID = rh.GetID()
	if err := gr.Create(db.Get(ctx)); err != nil {
		return nil, herrors.New(err, "can't create repo")
	}

	return &gr, nil
}

func DeactivateAll(ctx *context.C) error {
	userID, err := user.GetCurrentID(ctx.R)
	if err != nil {
		return fmt.Errorf("can't get current user id: %s", err)
	}

	// TODO: remove all hooks
	err = models.NewRepoQuerySet(db.Get(ctx)).
		UserIDEq(userID).Delete()
	if err != nil {
		return fmt.Errorf("can't delete all repos of user %d: %s", userID, err)
	}

	return nil
}

func ArePrivateReposEnabledForUser(ctx *context.C) bool {
	ga, err := user.GetAuth(ctx)
	if err != nil {
		errors.Errorf(ctx, "Can't get current auth: %s", err)
		return false
	}

	return ga.PrivateAccessToken != ""
}
