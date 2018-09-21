package repoanalyzes

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golib/server/context"
)

type NewReposLauncher struct {
	LastGithubRepoID uint

	LastRepoAnalysisStatusID uint
	repoToStatus             map[uint]models.RepoAnalysisStatus
}

func (nrl NewReposLauncher) Run() {
	nrl.repoToStatus = map[uint]models.RepoAnalysisStatus{}
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
		nrl.repoToStatus[as.RepoID] = as
	}

	var repos []models.Repo
	err = models.NewRepoQuerySet(db.Get(ctx)).
		IDGt(nrl.LastGithubRepoID).
		OrderDescByID().
		All(&repos)
	if err != nil {
		return fmt.Errorf("can't get repos: %s", err)
	}
	if len(repos) != 0 {
		nrl.LastGithubRepoID = repos[0].ID
	}

	for _, repo := range repos {
		_, ok := nrl.repoToStatus[repo.ID]
		if ok {
			continue
		}

		if err := nrl.createNewAnalysisStatusForRepo(ctx, &repo); err != nil {
			errors.Warnf(ctx, "Can't create new repo analysis status for %s: %s", repo.Name, err)
		}
		time.Sleep(time.Minute) // no more than 1 repo per minute
	}

	return nil
}

func (nrl *NewReposLauncher) createNewAnalysisStatusForRepo(ctx *context.C, repo *models.Repo) error {
	active := true
	f := NewGithubRepoStateFetcher(db.Get(ctx))
	state, err := f.Fetch(ctx.Ctx, repo)
	if err != nil {
		active = false
		errors.Warnf(ctx, "Create analysis for the new repo: mark repo as inactive: "+
			"can't fetch initial state for repo %s: %s", repo.Name, err)
		state = &GithubRepoState{}
	}

	as := models.RepoAnalysisStatus{
		DefaultBranch:     state.DefaultBranch,
		PendingCommitSHA:  state.HeadCommitSHA,
		HasPendingChanges: true,
		Active:            active,
		RepoID:            repo.ID,
	}
	if err = as.Create(db.Get(ctx)); err != nil {
		return fmt.Errorf("can't create analysis status in db: %s", err)
	}

	ctx.L.Infof("Created new repo analysis status: %#v", as)
	return nil
}
