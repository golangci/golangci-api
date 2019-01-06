package repohook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/pullanalyzesqueue"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
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
	PullAnalyzeQueue      *pullanalyzesqueue.Producer
	ActiveSubPolicy       *policy.ActiveSubscription
	Cfg                   config.Config
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

	return fmt.Errorf("got unknown github webhook event type %s, body: %s", eventType, string(body))
}

//nolint:gocyclo
func (s BasicService) handleGithubPullRequestWebhook(rc *request.AnonymousContext, repo *models.Repo,
	req *GithubWebhook, body request.Body) error {

	var auth models.Auth
	if err := models.NewAuthQuerySet(rc.DB).UserIDEq(repo.UserID).One(&auth); err != nil {
		return errors.Wrapf(err, "failed to get auth for repo %d", repo.ID)
	}

	p, err := s.ProviderFactory.Build(&auth)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for auth %d", auth.ID)
	}

	ev, err := p.ParsePullRequestEvent(rc.Ctx, body)
	if err != nil {
		return errors.Wrap(err, "failed to parse pull request event")
	}

	rc.Log.Infof("Got repo %s github pull request #%d webhook", repo.String(), ev.PullRequestNumber)

	if ev.Action != provider.Opened && ev.Action != provider.Synchronized {
		rc.Log.Infof("Got github webhook with action %s, skip it", ev.Action)
		return nil
	}

	if ev.Repo.IsPrivate {
		if err = s.ActiveSubPolicy.CheckForProviderPullRequestEvent(rc.Ctx, p, ev); err != nil {
			if errors.Cause(err) == policy.ErrNoActiveSubscription {
				rc.Log.Warnf("Got PR to %s with no active subscription: %s", repo.FullName, err)
				return nil // TODO(d.isaev): set proper error status and notify
			}

			if errors.Cause(err) == policy.ErrNoSeatInSubscription {
				rc.Log.Warnf("Got PR to %s without matched private seat: %s", repo.FullName, err)
				return nil // TODO(d.isaev): set proper error status and notify
			}

			return err
		}

		rc.Log.Infof("Got PR webhook to the private repo %s", repo.String())
	}

	// TODO: create pr analysis only if could set commit status
	analysis, err := s.createGithubPullRequestAnalysis(rc, ev, repo, req)
	if err != nil {
		return err
	}

	err = p.SetCommitStatus(rc.Ctx, repo.Owner(), repo.Repo(), analysis.CommitSHA, &provider.CommitStatus{
		State:       string(github.StatusPending),
		Description: "Waiting in queue...",
		Context:     s.Cfg.GetString("APP_NAME"),
	})
	if err != nil {
		if err == provider.ErrUnauthorized || err == provider.ErrNotFound {
			rc.Log.Warnf("Can't set github commit status to 'pending in queue', skipping webhook: %s", err)
			return nil
		}

		return errors.Wrap(err, "failed to set commit status")
	}

	githubCtx := github.Context{
		Repo: github.Repo{
			Owner: repo.Owner(),
			Name:  repo.Repo(),
		},
		GithubAccessToken: auth.StrongestAccessToken(),
		PullRequestNumber: analysis.PullRequestNumber,
	}

	msg := pullanalyzesqueue.RunMessage{
		Context:      githubCtx,
		UserID:       repo.UserID,
		AnalysisGUID: analysis.GithubDeliveryGUID,
	}
	if err = s.PullAnalyzeQueue.Put(&msg); err != nil {
		return errors.Wrap(err, "can't send pull request for analysis into queue: %s")
	}

	rc.Log.Infof("Sent task to pull request analyze queue")
	return nil
}

func (s BasicService) createGithubPullRequestAnalysis(rc *request.AnonymousContext, ev *provider.PullRequestEvent,
	repo *models.Repo, req *GithubWebhook) (*models.PullRequestAnalysis, error) {

	analysis := models.PullRequestAnalysis{
		RepoID:             repo.ID,
		PullRequestNumber:  ev.PullRequestNumber,
		GithubDeliveryGUID: req.DeliveryGUID,
		CommitSHA:          ev.Head.CommitSHA,

		Status:     "sent_to_queue",
		ResultJSON: []byte("{}"),
	}
	if err := analysis.Create(rc.DB); err != nil {
		return nil, errors.Wrap(err, "can't create analysis")
	}

	rc.Log.Infof("Created pr analysis id=%d", analysis.ID)
	return &analysis, nil
}

func (s BasicService) checkSubscription(rc *request.AnonymousContext, repo *models.Repo) error {
	var auth models.Auth
	if err := models.NewAuthQuerySet(rc.DB).UserIDEq(repo.UserID).One(&auth); err != nil {
		return errors.Wrapf(err, "failed to get auth for repo %d", repo.ID)
	}

	p, err := s.ProviderFactory.Build(&auth)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for auth %d", auth.ID)
	}

	pr, err := p.GetRepoByName(rc.Ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return errors.Wrap(err, "failed to get provider repo by name")
	}

	return s.ActiveSubPolicy.CheckForProviderRepo(p, pr)
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

	if payload.GetRepo().GetPrivate() {
		if err := s.checkSubscription(rc, repo); err != nil {
			// TODO: render message about inactive sub and send notification
			return errors.Wrap(err, "failed to check subscription")
		}
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
