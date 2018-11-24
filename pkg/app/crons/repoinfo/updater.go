package repoinfo

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Updater struct {
	DB  *gorm.DB
	Log logutil.Log
	Cfg config.Config
	Pf  providers.Factory
}

func (u Updater) Run() {
	timeout := u.Cfg.GetDuration("REPO_INFO_UPDATE_TIMEOUT", 60*time.Minute)

	lastErrors := map[uint]time.Time{}
	for range time.Tick(timeout / 2) {
		if err := u.runIteration(lastErrors); err != nil {
			u.Log.Warnf("Can't run iteration of updating repos: %s", err)
		}
	}
}

func (u Updater) runIteration(lastErrors map[uint]time.Time) error {
	var repos []models.Repo
	if err := models.NewRepoQuerySet(u.DB).OrderDescByID().All(&repos); err != nil {
		return errors.Wrap(err, "can't get repos")
	}

	errorsTimeout := u.Cfg.GetDuration("REPO_INFO_UPDATE_ERRORS_TIMEOUT", 24*60*time.Minute)
	printAllErrors := u.Cfg.GetBool("REPO_INFO_UPDATE_PRINT_ALL_ERRORS", false)

	var failedN int
	for _, r := range repos {
		if err := u.updateRepoInfo(&r); err != nil {
			failedN++
			if printAllErrors {
				u.Log.Warnf("Failed to update repo %s ID=%d info: %s", r.Name, r.ID, err)
			}

			cause := errors.Cause(err)
			if cause == provider.ErrUnauthorized || cause == provider.ErrNotFound {
				continue
			}

			if !printAllErrors {
				lastErroredAt, ok := lastErrors[r.ID]
				if ok && lastErroredAt.Add(errorsTimeout).Before(time.Now()) {
					u.Log.Warnf("Failed to update repo %s ID=%d info: %s", r.Name, r.ID, err)
					lastErrors[r.ID] = time.Now()
				}
			}
		}
	}

	u.Log.Infof("Updated repo info for %d repos, failed for %d repos", len(repos)-failedN, failedN)
	return nil
}

func (u Updater) updateRepoInfo(r *models.Repo) error {
	p, err := u.Pf.BuildForUser(u.DB, r.UserID)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	providerRepo, err := p.GetRepoByName(ctx, r.Owner(), r.Repo())
	if err != nil {
		return errors.Wrapf(err, "failed to get repo by name %s/%s", r.Owner(), r.Repo())
	}

	r.StargazersCount = providerRepo.StargazersCount
	if err = r.Update(u.DB, models.RepoDBSchema.StargazersCount); err != nil {
		return errors.Wrap(err, "failed to update stargazers count")
	}

	return nil
}
