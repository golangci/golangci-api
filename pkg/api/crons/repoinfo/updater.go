package repoinfo

import (
	"context"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
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
				u.Log.Warnf("Failed to update repo %s ID=%d info: %s", r.FullName, r.ID, err)
			}

			cause := errors.Cause(err)
			if cause == provider.ErrUnauthorized || cause == provider.ErrNotFound {
				continue
			}

			if !printAllErrors {
				lastErroredAt, ok := lastErrors[r.ID]
				if ok && lastErroredAt.Add(errorsTimeout).Before(time.Now()) {
					u.Log.Warnf("Failed to update repo %s ID=%d info: %s", r.FullName, r.ID, err)
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

	up := models.NewRepoQuerySet(u.DB).IDEq(r.ID).GetUpdater()
	var needUpdate bool

	if r.StargazersCount != providerRepo.StargazersCount {
		r.StargazersCount = providerRepo.StargazersCount
		up = up.SetStargazersCount(r.StargazersCount)
		needUpdate = true
	}

	if r.DisplayFullName != providerRepo.FullName {
		r.DisplayFullName = providerRepo.FullName
		up = up.SetDisplayFullName(r.DisplayFullName)
		needUpdate = true
	}

	lcName := strings.ToLower(providerRepo.FullName)
	if r.FullName != lcName {
		u.Log.Infof("Updating repo ID=%d name from %s to %s", r.ID, r.FullName, lcName)
		r.FullName = lcName
		up = up.SetFullName(r.FullName)
		needUpdate = true
	}

	if r.IsPrivate != providerRepo.IsPrivate {
		u.Log.Infof("Updating is_private from %t to %t for repo %s",
			r.IsPrivate, providerRepo.IsPrivate, r.FullName)
		r.IsPrivate = providerRepo.IsPrivate
		up = up.SetIsPrivate(r.IsPrivate)
		needUpdate = true
	}

	if needUpdate {
		if err = up.Update(); err != nil {
			return errors.Wrap(err, "failed to update stargazers count")
		}
	}

	return nil
}
