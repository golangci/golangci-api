package repoanalyzes

import (
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/internal/errors"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golib/server/context"
	uuid "github.com/satori/go.uuid"
)

// reanalyze each repo every reanalyzeInterval duration
var reanalyzeInterval = getDurationFromEnv("REPO_REANALYZE_INTERVAL", 30*time.Minute)

const lintersVersion = "v1.10.1"

func launchPendingRepoAnalyzes() {
	ctx := utils.NewBackgroundContext()

	checkInterval := getDurationFromEnv("REPO_REANALYZE_CHECK_INTERVAL", 30*time.Second)
	for range time.Tick(checkInterval) {
		if err := launchPendingRepoAnalyzesIter(ctx); err != nil {
			errors.Warnf(ctx, "Can't launch analyzes: %s", err)
			continue
		}
	}
}

func launchPendingRepoAnalyzesIter(ctx *context.C) error {
	var analysisStatuses []models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		HasPendingChangesEq(true).
		All(&analysisStatuses)
	if err != nil {
		return fmt.Errorf("can't get all analysis statuses: %s", err)
	}

	for _, as := range analysisStatuses {
		if err := launchPendingRepoAnalysisChecked(ctx, &as); err != nil {
			return err
		}
	}

	return nil
}

func launchPendingRepoAnalysisChecked(ctx *context.C, as *models.RepoAnalysisStatus) error {
	needAnalysis := as.LastAnalyzedAt.IsZero() || as.LastAnalyzedAt.Add(reanalyzeInterval).Before(time.Now())
	if !needAnalysis {
		ctx.L.Infof("No need to launch analysis for analysis status %v: last_analyzed=%s ago, reanalyze_interval=%s",
			as, time.Since(as.LastAnalyzedAt), reanalyzeInterval)
		return nil
	}

	if err := launchRepoAnalysis(ctx, as); err != nil {
		return fmt.Errorf("can't launch analysis %+v: %s", as, err)
	}

	ctx.L.Infof("Launched pending analysis for %s...", as.Name)
	return nil
}

func launchRepoAnalysis(ctx *context.C, as *models.RepoAnalysisStatus) (err error) {
	finishTx, err := db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer finishTx(&err)

	a := models.RepoAnalysis{
		RepoAnalysisStatusID: as.ID,
		AnalysisGUID:         uuid.NewV4().String(),
		Status:               "sent_to_queue",
		CommitSHA:            as.PendingCommitSHA,
		ResultJSON:           []byte("{}"),
		AttemptNumber:        1,
		LintersVersion:       lintersVersion,
	}
	if err = a.Create(db.Get(ctx)); err != nil {
		return fmt.Errorf("can't create repo analysis: %s", err)
	}

	t := &task.RepoAnalysis{
		Name:         strings.ToLower(as.Name),
		AnalysisGUID: a.AnalysisGUID,
		Branch:       as.DefaultBranch,
	}

	if err = analyzequeue.ScheduleRepoAnalysis(t); err != nil {
		return fmt.Errorf("can't send repo for analysis into queue: %s", err)
	}

	n, err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		NameEq(strings.ToLower(as.Name)).
		VersionEq(as.Version).
		GetUpdater().
		SetHasPendingChanges(false).
		SetPendingCommitSHA("").
		SetVersion(as.Version + 1).
		SetLastAnalyzedAt(time.Now().UTC()).
		UpdateNum()
	if err != nil {
		return fmt.Errorf("can't update repo analysis status after processing: %s", err)
	}
	if n == 0 {
		return fmt.Errorf("got race condition updating repo analysis status on version %d->%d",
			as.Version, as.Version+1)
	}

	return nil
}
