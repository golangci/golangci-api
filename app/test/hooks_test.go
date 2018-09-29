package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golangci/golangci-api/app/test/sharedtest"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/analyzes"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-worker/app/lib/github"
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

func TestStaleAnalyzes(t *testing.T) {
	r, _ := sharedtest.GetActivatedRepo(t)

	ctx := utils.NewBackgroundContext()
	err := models.NewPullRequestAnalysisQuerySet(db.Get(ctx)).Delete()
	assert.NoError(t, err)

	r.ExpectWebhook("pull_request", getTestPREvent()).Status(http.StatusOK)

	timeout := time.Second
	staleCount, err := analyzes.CheckStaleAnalyzes(ctx, timeout)
	assert.NoError(t, err)
	assert.Zero(t, staleCount)

	time.Sleep(timeout + time.Millisecond)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mc := github.NewMockClient(ctrl)
	any := gomock.Any()
	mc.EXPECT().GetPullRequest(any, any).AnyTimes().Return(&gh.PullRequest{}, nil)
	mc.EXPECT().SetCommitStatus(any, any, any, any, any, "").Return(nil)
	analyzes.GithubClient = mc
	defer func() {
		analyzes.GithubClient = github.NewMyClient()
	}()

	staleCount, err = analyzes.CheckStaleAnalyzes(ctx, timeout)
	assert.NoError(t, err)
	assert.Equal(t, 1, staleCount)
}
