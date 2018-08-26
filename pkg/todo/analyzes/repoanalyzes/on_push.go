package repoanalyzes

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golib/server/context"
)

type RepoAnalysisStartState struct {
	DefaultBranch string
	HeadCommitSHA string
}

func FetchStartStateForRepoAnalysis(ctx *context.C, repo *models.GithubRepo) (*RepoAnalysisStartState, error) {
	gc, _, err := github.GetClientForUser(ctx, repo.UserID)
	if err != nil {
		return nil, fmt.Errorf("can't get github client: %s", err)
	}

	gr, _, err := gc.Repositories.Get(ctx.Ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return nil, fmt.Errorf("can't get repo %s from github: %s", repo.Name, err)
	}

	defaultBranch := gr.GetDefaultBranch()
	grb, _, err := gc.Repositories.GetBranch(ctx.Ctx, repo.Owner(), repo.Repo(), defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("can't get github branch %s info: %s", defaultBranch, err)
	}

	return &RepoAnalysisStartState{
		DefaultBranch: defaultBranch,
		HeadCommitSHA: grb.GetCommit().GetSHA(),
	}, nil
}

func OnRepoMasterUpdated(ctx *context.C, repoName, defaultBranch, commitSHA string) error {
	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		NameEq(strings.ToLower(repoName)). // repoName is in original case
		One(&as)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			as = models.RepoAnalysisStatus{
				Name:   repoName,
				Active: true,
			}
			if err = as.Create(db.Get(ctx)); err != nil {
				return fmt.Errorf("can't create repo analysis status %+v: %s", as, err)
			}
		} else {
			return fmt.Errorf("can't fetch analysis status with name %s: %s", repoName, err)
		}
	}

	as.HasPendingChanges = true
	as.DefaultBranch = defaultBranch
	as.PendingCommitSHA = commitSHA
	err = as.Update(db.Get(ctx),
		models.RepoAnalysisStatusDBSchema.HasPendingChanges,
		models.RepoAnalysisStatusDBSchema.DefaultBranch,
		models.RepoAnalysisStatusDBSchema.PendingCommitSHA,
	)
	if err != nil {
		return fmt.Errorf("can't update has_pending_changes to true: %s", err)
	}

	ctx.L.Infof("Set has_pending_changes=true, default_branch=%s for repo %s analysis status",
		defaultBranch, repoName)

	return nil
}
