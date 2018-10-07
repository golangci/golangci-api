package repoanalyzes

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golib/server/context"
)

func reanalyzeByNewLinters() {
	ctx := utils.NewBackgroundContext()
	analysisStatusesCh := make(chan models.RepoAnalysisStatus, 1024)
	go reanalyzeFromCh(ctx, analysisStatusesCh)

	for range time.NewTicker(10 * time.Minute).C {
		var analysisStatuses []models.RepoAnalysisStatus
		err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
			LastAnalyzedLintersVersionNe(lintersVersion).
			HasPendingChangesEq(false).
			ActiveEq(true).
			All(&analysisStatuses)
		if err != nil {
			errors.Warnf(ctx, "Can't fetch analysis statuses")
			continue
		}
		if len(analysisStatuses) == 0 {
			ctx.L.Infof("No analysis statuses to reanalyze by new linters")
			break
		}

		ctx.L.Infof("Fetched %d analysis statuses to reanalyze by new linters", len(analysisStatuses))

		for _, as := range analysisStatuses {
			analysisStatusesCh <- as
		}
	}

	close(analysisStatusesCh)
}

func reanalyzeFromCh(ctx *context.C, analysisStatusesCh <-chan models.RepoAnalysisStatus) {
	const avgAnalysisTime = time.Minute
	const maxReanalyzeCapacity = 0.5
	reanalyzeInterval := time.Duration(float64(avgAnalysisTime) / maxReanalyzeCapacity)

	for as := range analysisStatusesCh {
		ctx.L.Infof("Starting reanalyzing repo %d by new linters...", as.RepoID)
		if err := reanalyzeAnalysisByNewLinters(ctx, &as); err != nil {
			errors.Warnf(ctx, "Can't reanalyze analysis status %#v: %s", as, err)
		}
		time.Sleep(reanalyzeInterval)
	}
}

//nolint
func reanalyzeAnalysisByNewLinters(ctx *context.C, as *models.RepoAnalysisStatus) error {
	// use Unscoped to fetch deleted repos
	var repo models.Repo
	if err := models.NewRepoQuerySet(db.Get(ctx).Unscoped()).IDEq(as.RepoID).One(&repo); err != nil {
		return fmt.Errorf("failed to fetch repo with id %d: %s", as.RepoID, err)
	}

	var a models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(ctx)).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID().
		One(&a)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // no analyzes yet, likely it's an empty repo
		}
		return fmt.Errorf("can't fetch last repo analysis for %s: %s", repo.Name, err)
	}

	if as.LastAnalyzedLintersVersion == "" {
		err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
			IDEq(as.ID).
			GetUpdater().
			SetLastAnalyzedLintersVersion(a.LintersVersion).
			Update()
		if err != nil {
			return fmt.Errorf("can't set last_analyzed_linters_version to %s: %s", a.LintersVersion, err)
		}

		as.LastAnalyzedLintersVersion = a.LintersVersion
		if as.LastAnalyzedLintersVersion == lintersVersion {
			return nil
		}
	}

	err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		IDEq(as.ID).
		GetUpdater().
		SetHasPendingChanges(true).
		SetPendingCommitSHA(a.CommitSHA).
		Update()
	if err != nil {
		return fmt.Errorf("can't update has_pending_changes to true: %s", err)
	}

	ctx.L.Infof("Marked repo %s for reanalysis by new linters: %s -> %s",
		repo.Name, as.LastAnalyzedLintersVersion, lintersVersion)
	return nil
}
