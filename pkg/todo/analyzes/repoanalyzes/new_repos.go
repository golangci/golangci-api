package repoanalyzes

import (
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golib/server/context"
)

type NewReposLauncher struct {
	LastGithubRepoID uint

	LastRepoAnalysisStatusID uint
	repoToStatus             map[string]models.RepoAnalysisStatus
}

func (nrl NewReposLauncher) Run() {
	nrl.repoToStatus = map[string]models.RepoAnalysisStatus{}
	ctx := utils.NewBackgroundContext()
	checkInterval := getDurationFromEnv("NEW_REPOS_ANALYZES_LAUNCH_INTERVAL", 10*time.Second)

	for range time.Tick(checkInterval) {
		if err := nrl.createNewAnalysisStatuses(ctx); err != nil {
			errors.Warnf(ctx, "Can't create new repos analyzes: %s", err)
		}
	}
}

func (nrl *NewReposLauncher) createNewAnalysisStatuses(ctx *context.C) error {
	var analysisStatuses []models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		IDGt(nrl.LastRepoAnalysisStatusID).
		OrderDescByID().
		All(&analysisStatuses)
	if err != nil {
		return fmt.Errorf("can't get analysis statuses: %s", err)
	}
	if len(analysisStatuses) != 0 {
		nrl.LastRepoAnalysisStatusID = analysisStatuses[0].ID
	}

	for _, as := range analysisStatuses {
		nrl.repoToStatus[strings.ToLower(as.Name)] = as
	}

	var githubRepos []models.GithubRepo
	err = models.NewGithubRepoQuerySet(db.Get(ctx)).
		IDGt(nrl.LastGithubRepoID).
		OrderDescByID().
		All(&githubRepos)
	if err != nil {
		return fmt.Errorf("can't get github repos: %s", err)
	}
	if len(githubRepos) != 0 {
		nrl.LastGithubRepoID = githubRepos[0].ID
	}

	for _, repo := range githubRepos {
		_, ok := nrl.repoToStatus[strings.ToLower(repo.Name)]
		if ok {
			continue
		}

		if err := nrl.createNewAnalysisStatusForRepo(ctx, &repo); err != nil {
			return fmt.Errorf("can't create repo analysis status for %s: %s", repo.Name, err)
		}
		time.Sleep(time.Minute) // no more than 1 repo per minute
	}

	return nil
}

func (nrl *NewReposLauncher) createNewAnalysisStatusForRepo(ctx *context.C, repo *models.GithubRepo) error {
	state, err := FetchStartStateForRepoAnalysis(ctx, repo)
	if err != nil {
		return err
	}

	as := models.RepoAnalysisStatus{
		Name:              strings.ToLower(repo.Name),
		DefaultBranch:     state.DefaultBranch,
		PendingCommitSHA:  state.HeadCommitSHA,
		HasPendingChanges: true,
	}
	if err := as.Create(db.Get(ctx)); err != nil {
		return fmt.Errorf("can't create analysis status in db: %s", err)
	}

	ctx.L.Infof("Created new repo analysis status: %#v", as)
	return nil
}
