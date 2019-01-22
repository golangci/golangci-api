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

var errSkipWehbook = errors.New("skip webhook")

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
			if errors.Cause(err) == errSkipWehbook {
				return nil
			}

			return errors.Wrapf(err, "failed to handle github %s webhook", eventType)
		}
		return nil
	case "push":
		if err := s.handleGithubPushWebhook(rc, &repo, body); err != nil {
			if errors.Cause(err) == errSkipWehbook {
				return nil
			}

			return errors.Wrapf(err, "failed to handle github %s webhook", eventType)
		}
		return nil
	}

	return fmt.Errorf("got unknown github webhook event type %s, body: %s", eventType, string(body))
}

//nolint:gocyclo
func (s BasicService) handleGithubPullRequestWebhook(rc *request.AnonymousContext, repo *models.Repo,
	req *GithubWebhook, body request.Body) error {

	skipRepos := s.Cfg.GetStringList("PULL_REQUEST_WEBHOOK_SKIP_REPOS")
	for _, sr := range skipRepos {
		if strings.EqualFold(sr, repo.FullName) {
			rc.Log.Infof("Skip webhook for repo %s because it's in ignore list", repo.FullName)
			return errSkipWehbook
		}
	}

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
		return errSkipWehbook
	}

	setCommitStatus := func(state github.Status, desc string) error {
		commitErr := p.SetCommitStatus(rc.Ctx, repo.Owner(), repo.Repo(), ev.Head.CommitSHA, &provider.CommitStatus{
			State:       string(state),
			Description: desc,
			Context:     s.Cfg.GetString("APP_NAME"),
		})
		if commitErr != nil {
			if commitErr == provider.ErrUnauthorized || commitErr == provider.ErrNotFound {
				rc.Log.Warnf("Can't set github commit status to '%s' for repo %s and commit %s, skipping webhook: %s",
					desc, repo.FullNameWithProvider(), ev.Head.CommitSHA, commitErr)
				return errSkipWehbook
			}

			return errors.Wrap(commitErr, "failed to set commit status")
		}

		return nil
	}

	accessToken := auth.AccessToken
	if ev.Repo.IsPrivate {
		if err = s.ActiveSubPolicy.CheckForProviderPullRequestEvent(rc.Ctx, p, ev); err != nil {
			logger := s.getNoSubWarnLogger(rc, repo)
			if errors.Cause(err) == policy.ErrNoActiveSubscription {
				logger("Got PR to %s with no active subscription, skip it and set commit status: %s", repo.FullName, err)
				return setCommitStatus(github.StatusError, "No active paid subscription for the private repo")
			}

			if errors.Cause(err) == policy.ErrNoSeatInSubscription {
				logger("Got PR to %s without matched private seat, skip it and set commit status: %s", repo.FullName, err)
				return setCommitStatus(github.StatusError, "Git author's email wasn't configured in GolangCI")
			}

			return err
		}

		if auth.PrivateAccessToken == "" {
			rc.Log.Errorf("Got PR to %s with no user private access token", repo.FullName)
			return setCommitStatus(github.StatusError, "No private repos access token")
		}
		accessToken = auth.PrivateAccessToken

		rc.Log.Infof("Got PR webhook to the private repo %s", repo.String())
	}

	// TODO: create pr analysis only if could set commit status
	analysis, err := s.createGithubPullRequestAnalysis(rc, ev, repo, req)
	if err != nil {
		return err
	}

	err = setCommitStatus(github.StatusPending, "Waiting in queue...")
	if err != nil {
		return err
	}

	githubCtx := github.Context{
		Repo: github.Repo{
			Owner:     repo.Owner(),
			Name:      repo.Repo(),
			IsPrivate: ev.Repo.IsPrivate, // TODO: sync ev.Repo with models.Repo
		},
		GithubAccessToken: accessToken,
		PullRequestNumber: analysis.PullRequestNumber,
	}

	msg := pullanalyzesqueue.RunMessage{
		Context:      githubCtx,
		UserID:       repo.UserID,
		AnalysisGUID: analysis.GithubDeliveryGUID,
	}
	if err = s.PullAnalyzeQueue.Put(&msg); err != nil {
		return errors.Wrap(err, "can't send pull request for analysis into queue")
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

func (s BasicService) getNoSubWarnLogger(rc *request.AnonymousContext, repo *models.Repo) logutil.Func {
	ignoredRepos := s.Cfg.GetStringList("KNOWN_PRIVATE_REPOS_WO_SUB")
	for _, ignoredRepo := range ignoredRepos {
		if strings.EqualFold(ignoredRepo, repo.FullName) {
			return rc.Log.Infof
		}
	}

	return rc.Log.Warnf
}

func (s BasicService) isNoSubError(err error) bool {
	causeErr := errors.Cause(err)
	return causeErr == policy.ErrNoActiveSubscription || causeErr == policy.ErrNoSeatInSubscription
}

func (s BasicService) handleGithubPushWebhook(rc *request.AnonymousContext, repo *models.Repo, body request.Body) error {
	var payload gh.PushEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		return errors.Wrapf(apierrors.ErrBadRequest, "invalid payload json: %s", err)
	}

	if payload.GetRepo().GetDefaultBranch() == "" {
		rc.Log.Errorf("Got push webhook without default branch, skip it: %+v, %+v",
			payload.GetRepo(), payload)
		return errSkipWehbook
	}

	if payload.GetRepo().GetPrivate() {
		if err := s.checkSubscription(rc, repo); err != nil {
			if s.isNoSubError(err) {
				s.getNoSubWarnLogger(rc, repo)(
					"Got push webhook to %s with no active subscription, skip it: %s",
					repo.FullName, err)

				return errSkipWehbook
			}

			// TODO: render message about inactive sub and send notification
			rc.Log.Warnf("Failed to check subscription for push webhook, retry it: %s", err)
			return err
		}

		rc.Log.Infof("Got push webhook to the private repo %s", repo.String())
	}

	branch := strings.TrimPrefix(payload.GetRef(), "refs/heads/")
	if branch != payload.GetRepo().GetDefaultBranch() {
		rc.Log.Infof("Got push webhook for branch %s, but default branch is %s, skip it",
			branch, payload.GetRepo().GetDefaultBranch())
		return errSkipWehbook
	}

	// TODO: update default branch if changed
	if err := s.AnalysisLauncherQueue.Put(repo.ID, payload.GetHeadCommit().GetID()); err != nil {
		return errors.Wrap(err, "failed to send to analyzes queue")
	}

	return nil
}
