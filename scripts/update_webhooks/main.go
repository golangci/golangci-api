package main

import (
	"fmt"
	"log"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/golangci/golib/server/context"
	gh "github.com/google/go-github/github"
	_ "github.com/mattes/migrate/database/postgres" // pg
)

func main() {
	if err := updateAllWebhooks(); err != nil {
		panic(err)
	}
}

func updateAllWebhooks() error {
	ctx := utils.NewBackgroundContext()

	var repos []models.Repo
	if err := models.NewRepoQuerySet(db.Get(ctx)).All(&repos); err != nil {
		return fmt.Errorf("can't fetch all repos: %s", err)
	}

	log.Printf("Got %d repos to update", len(repos))

	for i, repo := range repos {
		if err := updateRepoWebhook(ctx, &repo); err != nil {
			log.Printf("Can't update repo %#v webhook: %s", repo, err)
		}
		log.Printf("#%d/%d: successfully updated webhook", i+1, len(repos))
	}

	return nil
}

func updateRepoWebhook(ctx *context.C, repo *models.Repo) error {
	var ga models.Auth
	err := models.NewAuthQuerySet(db.Get(ctx)).
		UserIDEq(repo.UserID).
		One(&ga)
	if err != nil {
		return fmt.Errorf("can't get auth for user %d", repo.UserID)
	}

	gc := github.Context{GithubAccessToken: ga.AccessToken}.GetClient(ctx.Ctx) // public repos only
	_, _, err = gc.Repositories.EditHook(ctx.Ctx, repo.Owner(), repo.Repo(), repo.ProviderHookID, &gh.Hook{
		Events: []string{"push", "pull_request"},
	})
	if err != nil {
		return fmt.Errorf("can't edit webhook: %s", err)
	}

	return nil
}
