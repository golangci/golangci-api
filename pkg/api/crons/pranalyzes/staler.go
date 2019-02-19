package pranalyzes

import (
	"context"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Staler struct {
	Cfg             config.Config
	DB              *gorm.DB
	Log             logutil.Log
	ProviderFactory providers.Factory
}

func (r Staler) Run() {
	// If you change it don't forget to change it golangci-worker
	const taskProcessingTimeout = time.Minute * 10 * 12 // 12x as in golangci-worker: need time for queue processing

	for range time.Tick(taskProcessingTimeout / 2) {
		if _, err := r.RunIteration(taskProcessingTimeout); err != nil {
			r.Log.Warnf("Can't check stale analyzes: %s", err)
			continue
		}
	}
}

func (r Staler) RunIteration(taskProcessingTimeout time.Duration) (int, error) {
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
		}
	}

	return len(analyzes), nil
}

func (r Staler) setGithubStatus(ctx context.Context, analysis models.PullRequestAnalysis, repo *models.Repo) error {
	p, err := r.ProviderFactory.BuildForUser(r.DB, repo.UserID)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for user %d", repo.UserID)
	}

	pr, err := p.GetPullRequest(ctx, repo.Owner(), repo.Repo(), analysis.PullRequestNumber)
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			r.Log.Infof("Staler: unrecoverable error getting pull request %s#%d: %s", repo.String(), analysis.PullRequestNumber, err)
			return nil
		}

		return errors.Wrapf(err, "failed to get pull request %s#%d", repo.String(), analysis.PullRequestNumber)
	}

	err = p.SetCommitStatus(ctx, repo.Owner(), repo.Repo(), pr.Head.CommitSHA, &provider.CommitStatus{
		Description: "Processing timeout",
		State:       string(github.StatusError),
		Context:     r.Cfg.GetString("APP_NAME"),
	})
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			r.Log.Warnf("Staler: unrecoverable error setting github status to processing timeout for %s#%d: %s", repo.String(), analysis.PullRequestNumber, err)
			return nil
		}

		return errors.Wrap(err, "can't set github status")
	}

	return nil
}

func (r Staler) updateStaleAnalysis(analysis models.PullRequestAnalysis) error {
	var repo models.Repo
	if err := models.NewRepoQuerySet(r.DB.Unscoped()).IDEq(analysis.RepoID).One(&repo); err != nil {
		return errors.Wrap(err, "failed to fetch repo")
	}

	excludeRepos := r.Cfg.GetStringList("STALER_EXCLUDE_REPOS")
	for _, er := range excludeRepos {
		if strings.EqualFold(er, repo.FullName) {
			r.Log.Infof("Staler: exclude repo %s from staling", repo.FullNameWithProvider())
			return nil
		}
	}

	ctx := context.Background()
	if err := r.setGithubStatus(ctx, analysis, &repo); err != nil {
		return err
	}

	analysis.Status = "forced_stale"
	if err := analysis.Update(r.DB, models.PullRequestAnalysisDBSchema.Status); err != nil {
		return errors.Wrap(err, "can't update stale analysis")
	}

	r.Log.Warnf("Fixed stale analysis for repo %s: %+v", repo.String(), analysis)
	return nil
}
