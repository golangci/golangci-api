package repos

import (
	"encoding/json"
	"fmt"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-worker/app/analyze/analyzerqueue"
	"github.com/golangci/golangci-worker/app/analyze/task"
	"github.com/golangci/golangci-worker/app/utils/github"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	"github.com/golangci/golib/server/handlers/manager"
	gh "github.com/google/go-github/github"
)

func receiveGithubWebhook(ctx context.C) error {
	var payload gh.PullRequestEvent
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return herrors.New400Errorf("invalid payload json: %s", err)
	}

	if payload.PullRequest == nil {
		ctx.L.Infof("Got webhook without PR")
		return nil
	}

	if payload.GetAction() != "opened" && payload.GetAction() != "synchronize" {
		ctx.L.Infof("Got webhook with action %s, skip it", payload.GetAction())
		return nil
	}

	prNumber := payload.PullRequest.GetNumber()
	if prNumber == 0 {
		return fmt.Errorf("got zero pull request number: %+v", payload.PullRequest)
	}

	var gr models.GithubRepo
	hookID := ctx.URLVar("hookID")
	err := models.NewGithubRepoQuerySet(db.Get(&ctx)).
		HookIDEq(hookID).
		One(&gr)
	if err != nil {
		return fmt.Errorf("can't get github repo with hook id %q: %s", hookID, err)
	}

	if gr.Name != fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name")) {
		return herrors.New400Errorf("invalid reponame: expected %q", gr.Name)
	}

	var ga models.GithubAuth
	err = models.NewGithubAuthQuerySet(db.Get(&ctx)).
		UserIDEq(gr.UserID).
		One(&ga)
	if err != nil {
		return herrors.New(err, "can't get github auth for user %d", gr.UserID)
	}

	ctx.L.Infof("Got webhook %+v", payload)
	t := &task.Task{
		Context: github.Context{
			Repo: github.Repo{
				Owner: ctx.URLVar("owner"),
				Name:  ctx.URLVar("name"),
			},
			GithubAccessToken: ga.AccessToken,
			PullRequestNumber: prNumber,
		},
		APIRequestID: ctx.RequestID,
	}
	if err = analyzerqueue.Send(t); err != nil {
		return fmt.Errorf("can't send pull request for analysis into queue: %s", err)
	}

	ctx.L.Infof("Sent task %+v to analyze queue", t)
	return nil
}

func init() {
	manager.Register("/v1/repos/{owner}/{name}/hooks/{hookID}", receiveGithubWebhook)
}
