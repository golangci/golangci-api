package sharedtest

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golib/server/context"
	gh "github.com/google/go-github/github"
)

var initFakeGithubClientOnce sync.Once

func initFakeGithubClient() {
	initFakeGithubClientOnce.Do(func() {
		realGetClient := github.GetClient
		github.GetClient = func(ctx *context.C) (*gh.Client, bool, error) {
			client, private, err := realGetClient(ctx)
			if err != nil {
				return nil, false, err
			}

			u, err := url.Parse(fakeGithubServer.URL + "/")
			if err != nil {
				return nil, false, fmt.Errorf("can't parse fake github server url: %s", err)
			}

			client.BaseURL = u
			return client, private, nil
		}
	})
}
