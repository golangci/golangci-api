package repoanalyzes

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	apperrors "github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golib/server/context"
	"github.com/pkg/errors"
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
			apperrors.Warnf(ctx, "Can't launch analyzes: %s", err)
			continue
		}
	}
}

func launchPendingRepoAnalyzesIter(ctx *context.C) error {
	var analysisStatuses []models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx).Unscoped()).
		HasPendingChangesEq(true).
		All(&analysisStatuses)
	if err != nil {
		return fmt.Errorf("can't get all analysis statuses: %s", err)
	}

	sleepDuration := getDurationFromEnv("REPO_REANALYZE_SLEEP_DURATION", 2*time.Minute)
	for _, as := range analysisStatuses {
		if err := launchPendingRepoAnalysisChecked(ctx, &as); err != nil {
			apperrors.Warnf(ctx, "Can't launch pending analysis: %s", err)
		}

		time.Sleep(sleepDuration)
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

	// use Unscoped to fetch deleted repos
	var repo models.Repo
	if err := models.NewRepoQuerySet(db.Get(ctx).Unscoped()).IDEq(as.RepoID).One(&repo); err != nil {
		return errors.Wrapf(err, "failed to fetch repo with id %d", as.RepoID)
	}

	if err := launchRepoAnalysis(ctx, as, &repo); err != nil {
		return fmt.Errorf("can't launch analysis %+v: %s", as, err)
	}

	ctx.L.Infof("Launched pending analysis for %s...", repo.Name)
	return nil
}

//nolint:gocyclo
func launchRepoAnalysis(ctx *context.C, as *models.RepoAnalysisStatus, repo *models.Repo) (err error) {
	var finishTx db.FinishTxFunc
	finishTx, err = db.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer finishTx(&err)

	needSendToQueue := true
	nExisting, err := models.NewRepoAnalysisQuerySet(db.Get(ctx)).
		RepoAnalysisStatusIDEq(as.ID).CommitSHAEq(as.PendingCommitSHA).LintersVersionEq(lintersVersion).
		Count()
	if err != nil {
		return errors.Wrap(err, "can't count existing repo analyzes")
	}
	if nExisting != 0 {
		// TODO: just fix version on sending to queue
		apperrors.Warnf(ctx, "Can't create repo analysis because of "+
			"race condition with frequent pushes and not fixed commit in worker: %#v", *as)
		needSendToQueue = false
	}

	if needSendToQueue {
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
			Name:         repo.Name,
			AnalysisGUID: a.AnalysisGUID,
			Branch:       as.DefaultBranch,
		}

		if err = analyzequeue.ScheduleRepoAnalysis(t); err != nil {
			return fmt.Errorf("can't send repo for analysis into queue: %s", err)
		}
	}

	n, err := models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).
		IDEq(as.ID).
		VersionEq(as.Version).
		GetUpdater().
		SetHasPendingChanges(false).
		SetPendingCommitSHA("").
		SetVersion(as.Version + 1).
		SetLastAnalyzedAt(time.Now().UTC()).
		SetLastAnalyzedLintersVersion(lintersVersion).
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
