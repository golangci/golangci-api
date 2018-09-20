package repoanalyzes

import (
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golib/server/context"
)

func OnRepoMasterUpdated(ctx *context.C, repo *models.Repo, defaultBranch, commitSHA string) error {
	repoName := repo.Name
	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).RepoIDEq(repo.ID).One(&as)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			as = models.RepoAnalysisStatus{
				Active: true,
				RepoID: repo.ID,
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
