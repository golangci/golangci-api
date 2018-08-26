package repos

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
)

func DeactivateRepo(ctx *context.C, owner, repo string) (*models.GithubRepo, error) {
	gc, _, err := github.GetClient(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get github client")
	}

	var gr models.GithubRepo
	err = models.NewGithubRepoQuerySet(db.Get(ctx)).
		NameEq(fmt.Sprintf("%s/%s", owner, repo)).
		One(&gr)
	if err != nil {
		if err == gorm.ErrRecordNotFound { // Race condition: double deactivation request
			return &models.GithubRepo{}, nil
		}

		return nil, fmt.Errorf("can't get repo %s/%s: %s", owner, repo, err)
	}

	_, err = gc.Repositories.DeleteHook(ctx.Ctx, owner, repo, gr.GithubHookID)
	if err != nil {
		return nil, fmt.Errorf("can't delete hook %d from github repo %s/%s: %s",
			gr.GithubHookID, owner, repo, err)
	}

	if err = gr.Delete(db.Get(ctx)); err != nil {
		return nil, fmt.Errorf("can't delete github repo: %s", err)
	}

	return &gr, nil
}

func GetWebhookURLPathForRepo(name, hookID string) string {
	return fmt.Sprintf("/v1/repos/%s/hooks/%s", name, hookID)
}

func ActivateRepo(ctx *context.C, ga *models.GithubAuth, owner, repo string) (*models.GithubRepo, error) {
	repoName := fmt.Sprintf("%s/%s", owner, repo)

	var gr models.GithubRepo
	err := models.NewGithubRepoQuerySet(db.Get(ctx)).UserIDEq(ga.UserID).NameEq(repoName).One(&gr)
	if err == nil {
		ctx.L.Infof("user attempts to activate repo twice")
		return &gr, nil
	}

	err = models.NewGithubRepoQuerySet(db.Get(ctx)).UserIDNe(ga.UserID).NameEq(repoName).One(&gr)
	if err == nil {
		return nil, fmt.Errorf("repo is already activated by another user")
	}

	gc, _, err := github.GetClient(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get github client")
	}

	hookID, err := utils.GenerateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("can't generate hook id: %s", err)
	}

	gr = models.GithubRepo{
		UserID:      ga.UserID,
		Name:        repoName,
		DisplayName: repoName,
		HookID:      hookID,
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

	gr.GithubHookID = rh.GetID()
	if err := gr.Create(db.Get(ctx)); err != nil {
		return nil, herrors.New(err, "can't create github repo")
	}

	return &gr, nil
}

func DeactivateAll(ctx *context.C) error {
	userID, err := user.GetCurrentID(ctx)
	if err != nil {
		return fmt.Errorf("can't get current user id: %s", err)
	}

	// TODO: remove all hooks
	err = models.NewGithubRepoQuerySet(db.Get(ctx)).
		UserIDEq(userID).Delete()
	if err != nil {
		return fmt.Errorf("can't delete all repos of user %d: %s", userID, err)
	}

	return nil
}

func ArePrivateReposEnabledForUser(ctx *context.C) bool {
	ga, err := user.GetGithubAuth(ctx)
	if err != nil {
		errors.Errorf(ctx, "Can't get current github auth: %s", err)
		return false
	}

	return ga.PrivateAccessToken != ""
}
