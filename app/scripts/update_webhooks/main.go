package main

import (
	"fmt"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-worker/app/utils/github"
	"github.com/golangci/golib/server/context"
	gh "github.com/google/go-github/github"
)

func main() {
	if err := updateAllWebhooks(); err != nil {
		panic(err)
	}
}

func updateAllWebhooks() error {
	ctx := utils.NewBackgroundContext()

	var repos []models.GithubRepo
	if err := models.NewGithubRepoQuerySet(db.Get(ctx)).All(&repos); err != nil {
		return fmt.Errorf("can't fetch all repos: %s", err)
	}

	for _, repo := range repos {
		if err := updateRepoWebhook(ctx, &repo); err != nil {
			return fmt.Errorf("can't update repo %#v webhook: %s", repo, err)
		}
	}

	return nil
}

func updateRepoWebhook(ctx *context.C, repo *models.GithubRepo) error {
	var ga models.GithubAuth
	err := models.NewGithubAuthQuerySet(db.Get(ctx)).
		UserIDEq(repo.UserID).
		One(&ga)
	if err != nil {
		return fmt.Errorf("can't get github auth for user %d", repo.UserID)
	}

	gc := github.Context{GithubAccessToken: ga.AccessToken}.GetClient(ctx.Ctx) // public repos only
	_, _, err = gc.Repositories.EditHook(ctx.Ctx, repo.Owner(), repo.Repo(), repo.GithubHookID, &gh.Hook{
		Events: []string{"push", "pull_request"},
	})
	if err != nil {
		return fmt.Errorf("can't edit webhook: %s", err)
	}

	return nil
}
