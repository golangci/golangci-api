package repoanalyzes

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golib/server/context"
)

func restartBrokenRepoAnalyzes() {
	repoAnalysisTimeout := getDurationFromEnv("REPO_ANALYSIS_TIMEOUT", 60*time.Minute)
	ctx := utils.NewBackgroundContext()

	for range time.Tick(repoAnalysisTimeout / 2) {
		if err := restartBrokenRepoAnalyzesIter(ctx, repoAnalysisTimeout); err != nil {
			errors.Warnf(ctx, "Can't restart analyzes: %s", err)
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
		t := &task.RepoAnalysis{
			Name:         strings.ToLower(as.Name),
			AnalysisGUID: a.AnalysisGUID,
			Branch:       as.DefaultBranch,
		}

		a.AttemptNumber++
		err = a.Update(db.Get(ctx), models.RepoAnalysisDBSchema.AttemptNumber)
		if err != nil {
			return fmt.Errorf("can't update attempt number for analysis %+v: %s", a, err)
		}

		if err = analyzequeue.ScheduleRepoAnalysis(t); err != nil {
			return fmt.Errorf("can't resend repo %s for analysis into queue: %s", as.Name, err)
		}

		errors.Warnf(ctx, "Restarted analysis for %s in status %s with %d-th attempt",
			as.Name, a.Status, a.AttemptNumber)
	}

	return nil
}
