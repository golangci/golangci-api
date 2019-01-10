package repoanalyzes

import (
	"context"
	"math"
	"time"

	"github.com/golangci/golangci-api/internal/shared/providers"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/repoanalyzesqueue"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Restarter struct {
	DB       *gorm.DB
	Log      logutil.Log
	Cfg      config.Config
	RunQueue *repoanalyzesqueue.Producer
	Pf       providers.Factory
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

	// TODO: remove this Restarter completely and use SQS retry mechanism
	const maxAttemptsCount = 3

	for _, a := range analyzes {
		if a.AttemptNumber >= maxAttemptsCount {
			continue
		}

		retryTime := getNextRetryTime(&a)
		if retryTime.After(time.Now()) {
			continue
		}

		as := a.RepoAnalysisStatus

		var repo models.Repo
		if err := models.NewRepoQuerySet(r.DB.Unscoped()).IDEq(as.RepoID).One(&repo); err != nil {
			return errors.Wrapf(err, "failed to fetch repo with id %d", as.RepoID)
		}

		a.AttemptNumber++
		if err := a.Update(r.DB, models.RepoAnalysisDBSchema.AttemptNumber); err != nil {
			return errors.Wrapf(err, "can't update attempt number for analysis %+v", a)
		}

		privateAccessToken, err := r.getPrivateAccessToken(&repo)
		if err != nil {
			return errors.Wrap(err, "failed to get private access token")
		}

		if err := r.RunQueue.Put(repo.FullName, a.AnalysisGUID, as.DefaultBranch, privateAccessToken); err != nil {
			return errors.Wrapf(err, "can't resend repo %s for analysis into queue", repo.FullName)
		}

		r.Log.Warnf("Restarted repo analysis for %s in status %s with %d-th attempt",
			repo.FullName, a.Status, a.AttemptNumber)
	}

	return nil
}

func (r Restarter) getPrivateAccessToken(repo *models.Repo) (string, error) {
	var auth models.Auth
	if err := models.NewAuthQuerySet(r.DB).UserIDEq(repo.UserID).One(&auth); err != nil {
		return "", errors.Wrapf(err, "failed to fetch auth for user id %d", repo.UserID)
	}

	p, err := r.Pf.Build(&auth)
	if err != nil {
		return "", errors.Wrap(err, "failed to build provider for auth")
	}

	ctx := context.Background()
	providerRepo, err := p.GetRepoByName(ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch provider repo %s", repo.FullName)
	}

	if providerRepo.IsPrivate {
		return auth.PrivateAccessToken, nil
	}

	return "", nil
}
