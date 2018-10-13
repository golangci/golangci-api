package repohook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/pkg/providers/provider"

	"github.com/golangci/golangci-api/pkg/apierrors"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/providers"
	"github.com/golangci/golangci-api/pkg/request"

	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golangci-worker/app/lib/github"
	gh "github.com/google/go-github/github"
)

type GithubRepo struct {
	request.ShortRepo

	HookID string `request:",urlPart,"`
}

type Service interface {
	//url:/v1/repos/{owner}/{name}/hooks/{hookid} method:POST
	HandleGithubWebhook(rc *request.AnonymousContext, reqRepo *GithubRepo, body request.Body) error
}

type BasicService struct {
	ProviderFactory providers.Factory
}

func (s BasicService) HandleGithubWebhook(rc *request.AnonymousContext, reqRepo *GithubRepo, body request.Body) error {
	eventType := rc.Headers.Get("X-GitHub-Event")
	if eventType == "ping" {
		rc.Log.Infof("Got ping webhook")
		return nil
	}

	var repo models.Repo
	err := models.NewRepoQuerySet(rc.DB).HookIDEq(reqRepo.HookID).One(&repo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(apierrors.ErrNotFound, "no repo for hook id %s", reqRepo.HookID)
		}

		return errors.Wrapf(err, "can't get repo with hook id %q", reqRepo.HookID)
	}
	// reqRepo's owner and name are ignored

	switch eventType {
	case "pull_request":
		if err := s.handleGithubPullRequestWebhook(rc, &repo, body); err != nil {
			return errors.Wrapf(err, "failed to handle github %s webhook", eventType)
		}
		return nil
	case "push":
		if err := s.handleGithubPushWebhook(rc, &repo, body); err != nil {
			return errors.Wrapf(err, "failed to handle github %s webhook", eventType)
		}
		return nil
	}

	return fmt.Errorf("got unknown github webhook event type %s", eventType)
}

//nolint:gocyclo
func (s BasicService) handleGithubPullRequestWebhook(rc *request.AnonymousContext, repo *models.Repo, body request.Body) error {
	var payload gh.PullRequestEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return errors.Wrapf(apierrors.ErrBadRequest, "invalid payload json: %s", err)
	}

	if payload.PullRequest == nil {
		rc.Log.Infof("Got github webhook without PR")
		return nil
	}

	if payload.GetAction() != "opened" && payload.GetAction() != "synchronize" {
		rc.Log.Infof("Got github webhook with action %s, skip it", payload.GetAction())
		return nil
	}

	rc.Log.Infof("Got repo %s github pull request #%d webhook", repo.String(), payload.GetPullRequest().GetNumber())

	var auth models.Auth
	if err := models.NewAuthQuerySet(rc.DB).UserIDEq(repo.UserID).One(&auth); err != nil {
		return errors.Wrapf(err, "failed to get auth for repo %d", repo.ID)
	}

	taskAccessToken := auth.StrongestAccessToken()
	if payload.GetRepo().GetPrivate() && auth.PrivateAccessToken == "" {
		rc.Log.Errorf("Github repo %s became private: can't handle it without private access token", repo.String())
		return nil
	}

	// TODO: create pr analysis only if could set commit status
	analysis, err := s.createGithubPullRequestAnalysis(rc, payload.PullRequest, repo)
	if err != nil {
		return err
	}

	githubCtx := github.Context{
		Repo: github.Repo{
			Owner: repo.Owner(),
			Name:  repo.Repo(),
		},
		GithubAccessToken: taskAccessToken,
		PullRequestNumber: analysis.PullRequestNumber,
	}
	t := &task.PRAnalysis{
		Context:      githubCtx,
		UserID:       repo.UserID,
		AnalysisGUID: analysis.GithubDeliveryGUID,
	}

	p, err := s.ProviderFactory.Build(&auth)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for auth %d", auth.ID)
	}

	err = p.SetCommitStatus(rc.Ctx, repo.Owner(), repo.Repo(), analysis.CommitSHA, &provider.CommitStatus{
		State:       string(github.StatusPending),
		Description: "Waiting in queue...",
		Context:     "GolangCI",
	})
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			rc.Log.Warnf("Can't set github commit status to 'pending in queue' for task %+v, skipping webhook: %s",
				t, err)
			return nil
		}

		return errors.Wrap(err, "failed to set commit status")
	}

	if err = analyzequeue.SchedulePRAnalysis(t); err != nil {
		return errors.Wrap(err, "can't send pull request for analysis into queue: %s")
	}

	rc.Log.Infof("Sent task to pull request analyze queue")
	return nil
}

func (s BasicService) createGithubPullRequestAnalysis(rc *request.AnonymousContext, pr *gh.PullRequest,
	repo *models.Repo) (*models.PullRequestAnalysis, error) {
	guid := rc.Headers.Get("X-GitHub-Delivery")
	if guid == "" {
		return nil, errors.Wrap(apierrors.ErrBadRequest, "delivery without GUID")
	}

	analysis := models.PullRequestAnalysis{
		RepoID:             repo.ID,
		PullRequestNumber:  pr.GetNumber(),
		GithubDeliveryGUID: guid,
		CommitSHA:          pr.GetHead().GetSHA(),

		Status:     "sent_to_queue",
		ResultJSON: []byte("{}"),
	}
	if err := analysis.Create(rc.DB); err != nil {
		return nil, errors.Wrap(err, "can't create analysis")
	}

	rc.Log.Infof("Created pr analysis id=%d", analysis.ID)
	return &analysis, nil
}

func (s BasicService) handleGithubPushWebhook(rc *request.AnonymousContext, repo *models.Repo, body request.Body) error {
	var payload gh.PushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return errors.Wrapf(apierrors.ErrBadRequest, "invalid payload json: %s", err)
	}

	branch := strings.TrimPrefix(payload.GetRef(), "refs/heads/")
	if branch != payload.GetRepo().GetDefaultBranch() {
		rc.Log.Infof("Got push webhook for branch %s, but default branch is %s, skip it",
			branch, payload.GetRepo().GetDefaultBranch())
		return nil
	}

	if payload.GetRepo().GetDefaultBranch() == "" {
		rc.Log.Errorf("Got push webhook without default branch: %+v, %+v",
			payload.GetRepo(), payload)
		return nil
	}

	err := repoanalyzes.OnRepoMasterUpdated(rc.DB, rc.Log, repo,
		payload.GetRepo().GetDefaultBranch(), payload.GetHeadCommit().GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create repo analysis")
	}

	return nil
}
