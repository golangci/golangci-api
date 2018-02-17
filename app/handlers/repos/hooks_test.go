package repos

import (
	"net/http"
	"testing"

	gh "github.com/google/go-github/github"
)

func TestReceivePingWebhook(t *testing.T) {
	r, _ := getDeactivatedRepo(t)
	r.Activate()

	r.ExpectWebhook(gh.PingEvent{}).Status(http.StatusOK)
}

func TestReceivePullRequestOpenedWebhook(t *testing.T) {
	r, _ := getDeactivatedRepo(t)
	r.Activate()

	opened := "opened"
	prNumber := 7
	p := gh.PullRequestEvent{
		Action: &opened,
		PullRequest: &gh.PullRequest{
			Number: &prNumber,
		},
	}
	r.ExpectWebhook(p).Status(http.StatusOK)
}
