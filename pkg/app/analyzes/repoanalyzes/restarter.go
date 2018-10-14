package repoanalyzes

import (
	"math"
	"time"

	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
)

type Restarter struct {
	DB  *gorm.DB
	Log logutil.Log
	Cfg config.Config
}

func (r Restarter) Run() {
	repoAnalysisTimeout := r.Cfg.GetDuration("REPO_ANALYSIS_TIMEOUT", 60*time.Minute)

	for range time.Tick(repoAnalysisTimeout / 2) {
		if err := r.runIteration(repoAnalysisTimeout); err != nil {
			r.Log.Warnf("Can't run iteration of restarting of repo analyzes: %s", err)
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

func (r Restarter) runIteration(repoAnalysisTimeout time.Duration) error {
	var analyzes []models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(r.DB).
		StatusIn("sent_to_queue", "processing", "error").
		CreatedAtLt(time.Now().Add(-repoAnalysisTimeout)).
		PreloadRepoAnalysisStatus().
		All(&analyzes)
	if err != nil {
		return errors.Wrap(err, "can't get repo analyzes")
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
		if err := models.NewRepoQuerySet(r.DB.Unscoped()).IDEq(as.RepoID).One(&repo); err != nil {
			return errors.Wrapf(err, "failed to fetch repo with id %d", as.RepoID)
		}

		t := &task.RepoAnalysis{
			Name:         repo.Name,
			AnalysisGUID: a.AnalysisGUID,
			Branch:       as.DefaultBranch,
		}

		a.AttemptNumber++
		if err := a.Update(r.DB, models.RepoAnalysisDBSchema.AttemptNumber); err != nil {
			return errors.Wrapf(err, "can't update attempt number for analysis %+v", a)
		}

		if err := analyzequeue.ScheduleRepoAnalysis(t); err != nil {
			return errors.Wrapf(err, "can't resend repo %s for analysis into queue", repo.Name)
		}

		r.Log.Warnf("Restarted repo analysis for %s in status %s with %d-th attempt",
			repo.Name, a.Status, a.AttemptNumber)
	}

	return nil
}
