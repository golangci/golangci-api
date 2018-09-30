package analyzes

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/golangci/golib/server/context"
)

var GithubClient github.Client = github.NewMyClient()

func StartWatcher() {
	go watch()
}

func watch() {
	// If you change it don't forget to change it golangci-worker
	const taskProcessingTimeout = time.Minute * 40 // 4x as in golangci-worker: need time for queue processing
	ctx := utils.NewBackgroundContext()

	for range time.Tick(taskProcessingTimeout / 2) {
		if _, err := CheckStaleAnalyzes(ctx, taskProcessingTimeout); err != nil {
			errors.Warnf(ctx, "Can't check stale analyzes: %s", err)
			continue
		}
	}
}

func CheckStaleAnalyzes(ctx *context.C, taskProcessingTimeout time.Duration) (int, error) {
	var analyzes []models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(db.Get(ctx)).
		StatusIn("sent_to_queue", "processing").
		CreatedAtLt(time.Now().Add(-taskProcessingTimeout)).
		All(&analyzes)
	if err != nil {
		return 0, fmt.Errorf("can't get github analyzes: %s", err)
	}

	if len(analyzes) == 0 {
		return 0, nil
	}

	for _, analysis := range analyzes {
		if err = updateStaleAnalysis(ctx, analysis); err != nil {
			errors.Errorf(ctx, "Can't update stale analysis %+v: %s", analysis, err)
		} else {
			errors.Warnf(ctx, "Fixed stale analysis %+v", analysis)
		}
	}

	return len(analyzes), nil
}

func getGithubContextForAnalysis(ctx *context.C, analysis models.PullRequestAnalysis, repo *models.Repo) (*github.Context, error) {
	var ga models.Auth
	err := models.NewAuthQuerySet(db.Get(ctx)).
		UserIDEq(repo.UserID).
		One(&ga)
	if err != nil {
		return nil, fmt.Errorf("can't get auth for user %d: %s", repo.UserID, err)
	}

	return &github.Context{
		Repo: github.Repo{
			Owner: repo.Owner(),
			Name:  repo.Repo(),
		},
		GithubAccessToken: ga.AccessToken,
		PullRequestNumber: analysis.PullRequestNumber,
	}, nil
}

func setGithubStatus(ctx *context.C, analysis models.PullRequestAnalysis, repo *models.Repo) error {
	githubContext, err := getGithubContextForAnalysis(ctx, analysis, repo)
	if err != nil {
		return err
	}

	pr, err := GithubClient.GetPullRequest(ctx.Ctx, githubContext)
	if err != nil {
		if !github.IsRecoverableError(err) {
			errors.Warnf(ctx, "%s: %+v", err, githubContext)
			return nil
		}
		return fmt.Errorf("can't get pull request: %s", err)
	}

	err = GithubClient.SetCommitStatus(ctx.Ctx, githubContext, pr.GetHead().GetSHA(),
		github.StatusError, "Processing timeout", "")
	if err != nil {
		return fmt.Errorf("can't set github status: %s", err)
	}

	return nil
}

func updateStaleAnalysis(ctx *context.C, analysis models.PullRequestAnalysis) error {
	var repo models.Repo
	if err := models.NewRepoQuerySet(db.Get(ctx).Unscoped()).IDEq(analysis.RepoID).One(&repo); err != nil {
		return fmt.Errorf("failed to fetch repo: %s", err)
	}

	if err := setGithubStatus(ctx, analysis, &repo); err != nil {
		return err
	}

	analysis.Status = "forced_stale"
	if err := analysis.Update(db.Get(ctx), models.PullRequestAnalysisDBSchema.Status); err != nil {
		return fmt.Errorf("can't update stale analysis: %s", err)
	}

	return nil
}
