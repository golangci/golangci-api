package pranalyzes

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Restarter struct {
	DB              *gorm.DB
	Log             logutil.Log
	ProviderFactory providers.Factory
}

func (r Restarter) Run() {
	// If you change it don't forget to change it golangci-worker
	const taskProcessingTimeout = time.Minute * 40 // 4x as in golangci-worker: need time for queue processing

	for range time.Tick(taskProcessingTimeout / 2) {
		if _, err := r.RunIteration(taskProcessingTimeout); err != nil {
			r.Log.Warnf("Can't check stale analyzes: %s", err)
			continue
		}
	}
}

func (r Restarter) RunIteration(taskProcessingTimeout time.Duration) (int, error) {
	var analyzes []models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(r.DB).
		StatusIn("sent_to_queue", "processing").
		CreatedAtLt(time.Now().Add(-taskProcessingTimeout)).
		All(&analyzes)
	if err != nil {
		return 0, errors.Wrap(err, "can't get github analyzes")
	}

	if len(analyzes) == 0 {
		return 0, nil
	}

	for _, analysis := range analyzes {
		if err = r.updateStaleAnalysis(analysis); err != nil {
			r.Log.Errorf("Can't update stale analysis %+v: %s", analysis, err)
		} else {
			r.Log.Warnf("Fixed stale analysis %+v", analysis)
		}
	}

	return len(analyzes), nil
}

func (r Restarter) setGithubStatus(ctx context.Context, analysis models.PullRequestAnalysis, repo *models.Repo) error {
	p, err := r.ProviderFactory.BuildForUser(r.DB, repo.UserID)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for user %d", repo.UserID)
	}

	pr, err := p.GetPullRequest(ctx, repo.Owner(), repo.Repo(), analysis.PullRequestNumber)
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			r.Log.Warnf("Unrecoverable error getting pull request %s#%d: %s", repo.String(), analysis.PullRequestNumber, err)
			return nil
		}

		return errors.Wrapf(err, "failed to get pull request %s#%d", repo.String(), analysis.PullRequestNumber)
	}

	err = p.SetCommitStatus(ctx, repo.Owner(), repo.Repo(), pr.HeadCommitSHA, &provider.CommitStatus{
		Description: "Processing timeout",
		State:       string(github.StatusError),
		Context:     "GolangCI",
	})
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			r.Log.Warnf("Unrecoverable error setting github status to processing timeout for %s#%d: %s", repo.String(), analysis.PullRequestNumber, err)
			return nil
		}

		return errors.Wrap(err, "can't set github status")
	}

	return nil
}

func (r Restarter) updateStaleAnalysis(analysis models.PullRequestAnalysis) error {
	var repo models.Repo
	if err := models.NewRepoQuerySet(r.DB.Unscoped()).IDEq(analysis.RepoID).One(&repo); err != nil {
		return errors.Wrap(err, "failed to fetch repo")
	}

	ctx := context.Background()
	if err := r.setGithubStatus(ctx, analysis, &repo); err != nil {
		return err
	}

	analysis.Status = "forced_stale"
	if err := analysis.Update(r.DB, models.PullRequestAnalysisDBSchema.Status); err != nil {
		return errors.Wrap(err, "can't update stale analysis")
	}

	return nil
}
