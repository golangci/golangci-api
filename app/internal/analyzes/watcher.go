package analyzes

import (
	"fmt"
	"strings"
	"time"

	cntxt "context"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-worker/app/utils/github"
	"github.com/golangci/golib/server/context"
	"github.com/sirupsen/logrus"
)

func StartWatcher() {
	go watch()
}

func watch() {
	const taskProcessingTimeout = time.Minute * 5
	ctx := &context.C{
		Ctx: cntxt.Background(),
		L:   logrus.StandardLogger().WithField("ctx", "analyzes watcher"),
	}

	for range time.Tick(taskProcessingTimeout / 2) {
		if err := checkStaleAnalyzes(ctx, taskProcessingTimeout); err != nil {
			ctx.L.Warnf("Can't check stale analyzes: %s", err)
			continue
		}
	}
}

func checkStaleAnalyzes(ctx *context.C, taskProcessingTimeout time.Duration) error {
	var analyzes []models.GithubAnalysis
	err := models.NewGithubAnalysisQuerySet(db.Get(ctx)).
		StatusIn("sent_to_queue", "processing").
		PreloadGithubRepo().
		All(&analyzes)
	if err != nil {
		return fmt.Errorf("can't get github analyzes: %s", err)
	}

	if len(analyzes) == 0 {
		ctx.L.Infof("Found no stale analyzes")
		return nil
	}

	ctx.L.Infof("Found %d in progress analyzes", len(analyzes))

	for _, analysis := range analyzes {
		if !analysis.CreatedAt.Before(time.Now().Add(-taskProcessingTimeout)) {
			ctx.L.Infof("Analysis %+v is in progress, but isn't stale")
			continue
		}

		ctx.L.Warnf("Detected stale analysis: %+v", analysis)
		if err = updateStaleAnalysis(ctx, analysis); err != nil {
			return fmt.Errorf("can't update stale analysis: %s", err)
		}
	}

	return nil
}

func getGithubContextForAnalysis(ctx *context.C, analysis models.GithubAnalysis) (*github.Context, error) {
	if analysis.GithubRepo.UserID == 0 {
		return nil, fmt.Errorf("no github repo: %+v", analysis.GithubRepo)
	}

	var ga models.GithubAuth
	err := models.NewGithubAuthQuerySet(db.Get(ctx)).
		UserIDEq(analysis.GithubRepo.UserID).
		One(&ga)
	if err != nil {
		return nil, fmt.Errorf("can't get github auth for user %d: %s", analysis.GithubRepo.UserID, err)
	}

	parts := strings.SplitN(analysis.GithubRepo.Name, "/", 2)
	repoOwner, repoName := parts[0], parts[1]
	if repoOwner == "" || repoName == "" {
		return nil, fmt.Errorf("invalid repo name: %s", analysis.GithubRepo.Name)
	}

	return &github.Context{
		Repo: github.Repo{
			Owner: repoOwner,
			Name:  repoName,
		},
		GithubAccessToken: ga.AccessToken,
		PullRequestNumber: analysis.GithubPullRequestNumber,
	}, nil
}

func setGithubStatus(ctx *context.C, analysis models.GithubAnalysis) error {
	githubContext, err := getGithubContextForAnalysis(ctx, analysis)
	if err != nil {
		return err
	}

	gc := github.NewMyClient()
	pr, err := gc.GetPullRequest(ctx.Ctx, githubContext)
	if err != nil {
		if err == github.ErrPRNotFound {
			ctx.L.Warnf("No such pull request: %+v", githubContext)
			return nil
		}
		return fmt.Errorf("can't get pull request: %s", err)
	}

	err = gc.SetCommitStatus(ctx.Ctx, githubContext, pr.GetHead().GetSHA(),
		github.StatusSuccess, "No issues found!")
	if err != nil {
		return fmt.Errorf("can't set github status: %s", err)
	}

	return nil
}

func updateStaleAnalysis(ctx *context.C, analysis models.GithubAnalysis) error {
	if err := setGithubStatus(ctx, analysis); err != nil {
		return err
	}

	analysis.Status = "forced_stale"
	if err := analysis.Update(db.Get(ctx), models.GithubAnalysisDBSchema.Status); err != nil {
		return fmt.Errorf("can't update stale analysis: %s", err)
	}

	return nil
}
