package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	ghtodo "github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/golangci/golib/server/context"
	gh "github.com/google/go-github/github"
	_ "github.com/mattes/migrate/database/postgres" // pg
)

func main() {
	if err := fillRepoIDs(); err != nil {
		panic(err)
	}
}

func fillRepoIDs() error {
	ctx := utils.NewBackgroundContext()

	var repos []models.Repo
	if err := models.NewRepoQuerySet(db.Get(ctx)).ProviderIDEq(0).All(&repos); err != nil {
		return fmt.Errorf("can't fetch all repos: %s", err)
	}

	log.Printf("Got %d repos to update", len(repos))

	for i, repo := range repos {
		if err := updateRepo(ctx, &repo); err != nil {
			log.Printf("Can't update repo %#v: %s", repo, err)
			continue
		}
		log.Printf("#%d/%d: successfully updated repo id to %d",
			i+1, len(repos), repo.ProviderID)
	}

	return nil
}

func updateRepo(ctx *context.C, repo *models.Repo) error {
	gc, _, err := ghtodo.GetClientForUser(ctx, repo.UserID)
	if err != nil {
		return fmt.Errorf("can't get github client: %s", err)
	}

	gr, _, err := gc.Repositories.Get(ctx.Ctx, repo.Owner(), repo.Repo())
	if err != nil {
		if gr, err = getRepoByFallback(ctx, repo); err != nil {
			return fmt.Errorf("can't get repo: %s", err)
		}
		ctx.L.Warnf("Used fallback for repo %s fetching", repo)
	}

	err = models.NewRepoQuerySet(db.Get(ctx)).IDEq(repo.ID).GetUpdater().
		SetProviderID(gr.GetID()).Update()
	if err != nil {
		return fmt.Errorf("can't update repo %#v id to %d: %s", repo, gr.GetID(), err)
	}

	repo.ProviderID = gr.GetID()
	return nil
}

func getRepoByFallback(ctx *context.C, repo *models.Repo) (*gh.Repository, error) {
	fallbackAccessToken := os.Getenv("GITHUB_FALLBACK_ACCESS_TOKEN")
	if fallbackAccessToken == "" {
		return nil, errors.New("no fallback github access token")
	}

	gc := github.Context{GithubAccessToken: fallbackAccessToken}.GetClient(ctx.Ctx)
	gr, _, err := gc.Repositories.Get(ctx.Ctx, repo.Owner(), repo.Repo())

	return gr, err
}
