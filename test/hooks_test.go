package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golangci/golangci-api/pkg/app/crons/pranalyzes"
	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/test/sharedtest"
	gh "github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
)

func TestReceivePingWebhook(t *testing.T) {
	r, _ := sharedtest.GetActivatedRepo(t)
	r.ExpectWebhook("ping", gh.PingEvent{}).Status(http.StatusOK)
}

func getTestPREvent() gh.PullRequestEvent {
	return gh.PullRequestEvent{
		Action: gh.String("opened"),
		PullRequest: &gh.PullRequest{
			Number: gh.Int(1),
			Head: &gh.PullRequestBranch{
				SHA: gh.String(fmt.Sprintf("sha_%d", time.Now().UnixNano())),
			},
		},
	}
}

func TestReceivePullRequestOpenedWebhook(t *testing.T) {
	r, _ := sharedtest.GetActivatedRepo(t)
	r.ExpectWebhook("pull_request", getTestPREvent()).Status(http.StatusOK)
}

func TestReceivePushWebhook(t *testing.T) {
	r, _ := sharedtest.GetActivatedRepo(t)
	r.ExpectWebhook("push", gh.PushEvent{
		Ref: gh.String("refs/heads/master"),
		Repo: &gh.PushEventRepository{
			DefaultBranch: gh.String("master"),
		},
		HeadCommit: &gh.PushEventCommit{
			ID: gh.String("sha"),
		},
	}).Status(http.StatusOK)
}

func TestStaleAnalyzes(t *testing.T) {
	r, _ := sharedtest.GetActivatedRepo(t)
	deps := sharedtest.GetDefaultTestApp().BuildCommonDeps()

	err := models.NewPullRequestAnalysisQuerySet(deps.DB).Delete()
	assert.NoError(t, err)

	r.ExpectWebhook("pull_request", getTestPREvent()).Status(http.StatusOK)

	timeout := 3 * time.Second
	staler := pranalyzes.Staler{
		DB:              deps.DB,
		Log:             deps.Log,
		ProviderFactory: deps.ProviderFactory,
	}
	staleCount, err := staler.RunIteration(timeout)
	assert.NoError(t, err)
	assert.Zero(t, staleCount)

	time.Sleep(timeout + time.Millisecond)

	staleCount, err = staler.RunIteration(timeout)
	assert.NoError(t, err)
	assert.Equal(t, 1, staleCount)
}
