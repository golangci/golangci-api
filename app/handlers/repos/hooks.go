package repos

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
)

func getPullRequestWebhookPayload(ctx context.C) (*gh.PullRequestEvent, error) {
	var payload gh.PullRequestEvent
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return nil, herrors.New400Errorf("invalid payload json: %s", err)
	}

	if payload.PullRequest == nil {
		ctx.L.Infof("Got webhook without PR")
		return nil, nil
	}

	if payload.GetAction() != "opened" && payload.GetAction() != "synchronize" {
		ctx.L.Infof("Got webhook with action %s, skip it", payload.GetAction())
		return nil, nil
	}

	if payload.PullRequest.GetNumber() == 0 {
		return nil, herrors.New400Errorf("got zero pull request number: %+v", payload.PullRequest)
	}

	return &payload, nil
}

func fetchRepo(ctx context.C) (*models.Repo, error) {
	var gr models.Repo
	hookID := ctx.URLVar("hookID")
	err := models.NewRepoQuerySet(db.Get(&ctx)).
		HookIDEq(hookID).
		One(&gr)
	if err != nil {
		return nil, fmt.Errorf("can't get repo with hook id %q: %s", hookID, err)
	}

	if gr.Name != strings.ToLower(fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name"))) {
		return nil, herrors.New400Errorf("invalid reponame: expected %q", gr.Name)
	}

	return &gr, nil
}

func createAnalysis(ctx context.C, pr *gh.PullRequest, gr *models.Repo) (*models.PullRequestAnalysis, error) {
	guid := ctx.R.Header.Get("X-GitHub-Delivery")
	if guid == "" {
		return nil, herrors.New400Errorf("delivery without GUID")
	}

	analysis := models.PullRequestAnalysis{
		RepoID:             gr.ID,
		PullRequestNumber:  pr.GetNumber(),
		GithubDeliveryGUID: guid,
		CommitSHA:          pr.GetHead().GetSHA(),

		Status:     "sent_to_queue",
		ResultJSON: []byte("{}"),
	}
	if err := analysis.Create(db.Get(&ctx)); err != nil {
		return nil, herrors.New(err, "can't create analysis")
	}

	return &analysis, nil
}

func receiveGithubWebhook(ctx context.C) error {
	eventType := ctx.R.Header.Get("X-GitHub-Event")
	switch eventType {
	case "pull_request":
		return receivePullRequestWebhook(ctx)
	case "push":
		return receivePushWebhook(ctx)
	case "ping":
		ctx.L.Infof("Got ping hook")
		return nil
	}

	return fmt.Errorf("got unknown webhook event type %s", eventType)
}

func receivePullRequestWebhook(ctx context.C) error {
	payload, err := getPullRequestWebhookPayload(ctx)
	if payload == nil {
		return err
	}
	ctx.L.Infof("Got webhook %+v", payload)

	gr, err := fetchRepo(ctx)
	if err != nil {
		return err
	}

	var ga models.Auth
	err = models.NewAuthQuerySet(db.Get(&ctx)).
		UserIDEq(gr.UserID).
		OrderDescByID().
		One(&ga)
	if err != nil {
		return herrors.New(err, "can't get auth for user %d", gr.UserID)
	}

	analysis, err := createAnalysis(ctx, payload.PullRequest, gr)
	if err != nil {
		return err
	}
	ctx.L.Infof("Analysis object is %+v", analysis)

	var taskAccessToken string
	if payload.GetRepo().GetPrivate() {
		taskAccessToken = ga.PrivateAccessToken
	} else {
		taskAccessToken = ga.AccessToken
	}

	githubCtx := github.Context{
		Repo: github.Repo{
			Owner: strings.ToLower(ctx.URLVar("owner")),
			Name:  strings.ToLower(ctx.URLVar("name")),
		},
		GithubAccessToken: taskAccessToken,
		PullRequestNumber: analysis.PullRequestNumber,
	}
	t := &task.PRAnalysis{
		Context:      githubCtx,
		APIRequestID: ctx.RequestID,
		UserID:       gr.UserID,
		AnalysisGUID: analysis.GithubDeliveryGUID,
	}

	gc := github.NewMyClient()
	err = gc.SetCommitStatus(ctx.Ctx, &githubCtx, analysis.CommitSHA,
		github.StatusPending, "Waiting in queue...", "")
	if err != nil {
		ctx.L.Infof("Can't set github commit status to 'pending in queue' for task %+v, skipping webhook: %s",
			t, err)
		if !github.IsRecoverableError(err) {
			return nil
		}
	}

	if err = analyzequeue.SchedulePRAnalysis(t); err != nil {
		return fmt.Errorf("can't send pull request for analysis into queue: %s", err)
	}
	ctx.L.Infof("Sent task %+v to analyze queue", t)

	return nil
}

func getPushWebhookPayload(ctx context.C) (*gh.PushEvent, error) {
	var payload gh.PushEvent
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return nil, herrors.New400Errorf("invalid payload json: %s", err)
	}

	branch := strings.TrimPrefix(payload.GetRef(), "refs/heads/")
	if branch != payload.GetRepo().GetDefaultBranch() {
		ctx.L.Infof("Got push webhook for branch %s, but default branch is %s, skip it",
			branch, payload.GetRepo().GetDefaultBranch())
		return nil, nil
	}

	return &payload, nil
}

func receivePushWebhook(ctx context.C) error {
	payload, err := getPushWebhookPayload(ctx)
	if payload == nil {
		return err
	}

	if payload.GetRepo().GetDefaultBranch() == "" {
		errors.Warnf(&ctx, "Got push webhook without default branch: %+v, %+v",
			payload.GetRepo(), *payload)
	}

	repo, err := fetchRepo(ctx)
	if err != nil {
		return err
	}

	return repoanalyzes.OnRepoMasterUpdated(&ctx, repo,
		payload.GetRepo().GetDefaultBranch(), payload.GetHeadCommit().GetID())
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/hooks/{hookID}", receiveGithubWebhook)
}
