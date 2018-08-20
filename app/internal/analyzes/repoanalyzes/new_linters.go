package repoanalyzes

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/internal/errors"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golib/server/context"
)

func reanalyzeByNewLinters() {
	ctx := utils.NewBackgroundContext()
	analysisStatusesCh := make(chan models.RepoAnalysisStatus, 1024)
	go reanalyzeFromCh(ctx, analysisStatusesCh)

	for range time.NewTicker(time.Minute).C {
		var analysisStatuses []models.RepoAnalysisStatus
		err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
			LastAnalyzedLintersVersionNe(lintersVersion).
			HasPendingChangesEq(false).
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
		if err := reanalyzeAnalysisByNewLinters(ctx, &as); err != nil {
			errors.Warnf(ctx, "Can't reanalyze analysis status %#v: %s", as, err)
		}
		time.Sleep(reanalyzeInterval)
	}
}

//nolint
func reanalyzeAnalysisByNewLinters(ctx *context.C, as *models.RepoAnalysisStatus) error {
	var a models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(ctx)).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID().
		One(&a)
	if err != nil {
		if err == gorm.ErrRecordNotFound { // TODO: remove this branch after first reanalysis of all repos
			if as.PendingCommitSHA == "" {
				var gr models.GithubRepo
				err = models.NewGithubRepoQuerySet(db.Get(ctx)).NameEq(as.Name).One(&gr)
				if err != nil {
					return fmt.Errorf("can't fetch github repo with name %s: %s", as.Name, err)
				}

				state, err := FetchStartStateForRepoAnalysis(ctx, &gr)
				if err != nil {
					return fmt.Errorf("can't fetch initial state for repo %s: %s", as.Name, err)
				}

				err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
					IDEq(as.ID).
					GetUpdater().
					SetHasPendingChanges(true).
					SetPendingCommitSHA(state.HeadCommitSHA).
					SetDefaultBranch(state.DefaultBranch).
					Update()
				if err != nil {
					return fmt.Errorf("can't set has_pending_changes to true: %s", err)
				}

				return nil
			}

			err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
				IDEq(as.ID).
				GetUpdater().
				SetHasPendingChanges(true).
				Update()
			if err != nil {
				return fmt.Errorf("can't set has_pending_changes to true: %s", err)
			}

			return nil
		}

		return fmt.Errorf("can't fetch last repo analysis for %s: %s", as.Name, err)
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
		as.Name, as.LastAnalyzedLintersVersion, lintersVersion)
	return nil
}
