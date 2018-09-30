package repoanalyzes

import (
	"fmt"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	apperrors "github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golib/server/context"
)

func restartBrokenRepoAnalyzes() {
	repoAnalysisTimeout := getDurationFromEnv("REPO_ANALYSIS_TIMEOUT", 60*time.Minute)
	ctx := utils.NewBackgroundContext()

	for range time.Tick(repoAnalysisTimeout / 2) {
		if err := restartBrokenRepoAnalyzesIter(ctx, repoAnalysisTimeout); err != nil {
			apperrors.Warnf(ctx, "Can't restart analyzes: %s", err)
		}
	}
}

func getNextRetryTime(a *models.RepoAnalysis) time.Time {
	const maxRetryInterval = time.Hour * 24

	// 1 => 2**1 = 2 => 1h
	// 2 => 2**2 = 4 => 2h
	// 3 => 2**3 = 8 => 4h
	// 4 => 2**4 = 16 => 8h
	// 5 => 2**5 = 32 => 16h

	retryInterval := time.Hour * time.Duration(math.Exp2(float64(a.AttemptNumber))) / 2
	if retryInterval > maxRetryInterval {
		retryInterval = maxRetryInterval
	}

	return a.UpdatedAt.Add(retryInterval)
}

func restartBrokenRepoAnalyzesIter(ctx *context.C, repoAnalysisTimeout time.Duration) error {
	var analyzes []models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(ctx)).
		StatusIn("sent_to_queue", "processing", "error").
		CreatedAtLt(time.Now().Add(-repoAnalysisTimeout)).
		PreloadRepoAnalysisStatus().
		All(&analyzes)
	if err != nil {
		return fmt.Errorf("can't get repo analyzes: %s", err)
	}

	if len(analyzes) == 0 {
		return nil
	}

	for _, a := range analyzes {
		retryTime := getNextRetryTime(&a)
		if retryTime.After(time.Now()) {
			continue
		}

		as := a.RepoAnalysisStatus

		var repo models.Repo
		if err := models.NewRepoQuerySet(db.Get(ctx).Unscoped()).IDEq(as.RepoID).One(&repo); err != nil {
			return errors.Wrapf(err, "failed to fetch repo with id %d", as.RepoID)
		}

		t := &task.RepoAnalysis{
			Name:         repo.Name,
			AnalysisGUID: a.AnalysisGUID,
			Branch:       as.DefaultBranch,
		}

		a.AttemptNumber++
		if err := a.Update(db.Get(ctx), models.RepoAnalysisDBSchema.AttemptNumber); err != nil {
			return errors.Wrapf(err, "can't update attempt number for analysis %+v", a)
		}

		if err := analyzequeue.ScheduleRepoAnalysis(t); err != nil {
			return errors.Wrapf(err, "can't resend repo %s for analysis into queue", repo.Name)
		}

		apperrors.Warnf(ctx, "Restarted analysis for %s in status %s with %d-th attempt",
			repo.Name, a.Status, a.AttemptNumber)
	}

	return nil
}
