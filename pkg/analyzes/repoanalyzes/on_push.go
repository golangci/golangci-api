package repoanalyzes

import (
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-shared/pkg/logutil"
)

func OnRepoMasterUpdated(db *gorm.DB, log logutil.Log, repo *models.Repo, defaultBranch, commitSHA string) error {
	repoName := repo.Name
	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(repo.ID).One(&as)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			as = models.RepoAnalysisStatus{
				Active: true,
				RepoID: repo.ID,
			}
			if err = as.Create(db); err != nil {
				return errors.Wrapf(err, "can't create repo analysis status %+v", as)
			}
		} else {
			return errors.Wrapf(err, "can't fetch analysis status with name %s", repoName)
		}
	}

	as.HasPendingChanges = true
	as.DefaultBranch = defaultBranch
	as.PendingCommitSHA = commitSHA
	err = as.Update(db,
		models.RepoAnalysisStatusDBSchema.HasPendingChanges,
		models.RepoAnalysisStatusDBSchema.DefaultBranch,
		models.RepoAnalysisStatusDBSchema.PendingCommitSHA,
	)
	if err != nil {
		return errors.Wrap(err, "can't update has_pending_changes to true")
	}

	log.Infof("Set has_pending_changes=true, default_branch=%s for repo %s analysis status",
		defaultBranch, repoName)
	return nil
}
