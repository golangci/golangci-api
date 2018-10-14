package repohook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/pkg/app/providers/provider"

	"github.com/golangci/golangci-api/pkg/endpoint/apierrors"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/endpoint/request"

	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golangci-worker/app/lib/github"
	gh "github.com/google/go-github/github"
)

type GithubWebhook struct {
	request.ShortRepo

	HookID       string `request:",urlPart,"`
	EventType    string `request:"X-GitHub-Event,header,"`
	DeliveryGUID string `request:"X-GitHub-Delivery,header,"`
}

func (w GithubWebhook) FillLogContext(lctx logutil.Context) {
	w.ShortRepo.FillLogContext(lctx)
	lctx["event_type"] = w.EventType
	lctx["delivery_guid"] = w.DeliveryGUID
}

type Service interface {
	//url:/v1/repos/{owner}/{name}/hooks/{hookid} method:POST
	HandleGithubWebhook(rc *request.AnonymousContext, reqRepo *GithubWebhook, body request.Body) error
}

type BasicService struct {
	ProviderFactory       providers.Factory
	AnalysisLauncherQueue *repoanalyzes.LauncherProducer
}

func (s BasicService) HandleGithubWebhook(rc *request.AnonymousContext, req *GithubWebhook, body request.Body) error {
	eventType := req.EventType
	if eventType == "ping" {
		rc.Log.Infof("Got ping webhook")
		return nil
	}

	var repo models.Repo
	err := models.NewRepoQuerySet(rc.DB).HookIDEq(req.HookID).One(&repo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(apierrors.ErrNotFound, "no repo for hook id %s", req.HookID)
		}

		return errors.Wrapf(err, "can't get repo with hook id %q", req.HookID)
	}
	// req's owner and name are ignored

	switch eventType {
	case "pull_request":
		if err := s.handleGithubPullRequestWebhook(rc, &repo, req, body); err != nil {
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
func (s BasicService) handleGithubPullRequestWebhook(rc *request.AnonymousContext, repo *models.Repo,
	req *GithubWebhook, body request.Body) error {

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
	analysis, err := s.createGithubPullRequestAnalysis(rc, payload.PullRequest, repo, req)
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
	repo *models.Repo, req *GithubWebhook) (*models.PullRequestAnalysis, error) {

	analysis := models.PullRequestAnalysis{
		RepoID:             repo.ID,
		PullRequestNumber:  pr.GetNumber(),
		GithubDeliveryGUID: req.DeliveryGUID,
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

	if payload.GetRepo().GetDefaultBranch() == "" {
		rc.Log.Errorf("Got push webhook without default branch: %+v, %+v",
			payload.GetRepo(), payload)
		return nil
	}

	branch := strings.TrimPrefix(payload.GetRef(), "refs/heads/")
	if branch != payload.GetRepo().GetDefaultBranch() {
		rc.Log.Infof("Got push webhook for branch %s, but default branch is %s, skip it",
			branch, payload.GetRepo().GetDefaultBranch())
		return nil
	}

	// TODO: update default branch if changed
	if err := s.AnalysisLauncherQueue.Put(repo.ID, payload.GetHeadCommit().GetID()); err != nil {
		return errors.Wrap(err, "failed to send to analyzes queue")
	}

	return nil
}
