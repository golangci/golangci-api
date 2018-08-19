package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/golangci/golangci-api/app/internal/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golib/server/context"
	_ "github.com/mattes/migrate/database/postgres" // pg
	"github.com/sirupsen/logrus"
)

func main() {
	if err := updateAllBranches(); err != nil {
		panic(err)
	}
}

func updateAllBranches() error {
	ctx := utils.NewBackgroundContext()

	var repos []models.GithubRepo
	if err := models.NewGithubRepoQuerySet(db.Get(ctx)).All(&repos); err != nil {
		return fmt.Errorf("can't fetch all repos: %s", err)
	}

	log.Printf("Got %d repos to update", len(repos))

	for i, repo := range repos {
		if err := updateRepoDefaultBranch(ctx, &repo); err != nil {
			log.Printf("Can't update repo %#v default branch: %s", repo, err)
		}
		log.Printf("#%d/%d: successfully updated default branch", i+1, len(repos))
	}

	return nil
}

func updateRepoDefaultBranch(ctx *context.C, repo *models.GithubRepo) error {
	state, err := repoanalyzes.FetchStartStateForRepoAnalysis(ctx, repo)
	if err != nil {
		return err
	}

	var as models.RepoAnalysisStatus
	err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		NameEq(strings.ToLower(repo.Name)).
		One(&as)
	if err != nil {
		return fmt.Errorf("can't get repo analysis status for %s: %s", repo.Name, err)
	}

	if as.DefaultBranch != state.DefaultBranch {
		logrus.Infof("Changing %s default branch: %s -> %s", as.DefaultBranch, state.DefaultBranch)
	}

	err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		NameEq(as.Name).
		GetUpdater().
		SetDefaultBranch(state.DefaultBranch).
		SetPendingCommitSHA(state.HeadCommitSHA).
		SetHasPendingChanges(true).
		Update()
	if err != nil {
		return fmt.Errorf("can't update analysis status in db: %s", err)
	}

	return nil
}
