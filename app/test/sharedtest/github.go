package sharedtest

import (
	"net/url"
	"sync"

	"github.com/golangci/golangci-api/pkg/providers/provider"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golib/server/context"
	gh "github.com/google/go-github/github"
	"github.com/pkg/errors"
)

var initFakeGithubClientOnce sync.Once

func initFakeGithubClient() {
	initFakeGithubClientOnce.Do(func() {
		realGetClient := github.GetClient
		baseURL := fakeGithubServer.URL + "/"
		github.GetClient = func(ctx *context.C) (*gh.Client, bool, error) {
			client, private, err := realGetClient(ctx)
			if err != nil {
				return nil, false, err
			}

			u, err := url.Parse(baseURL)
			if err != nil {
				return nil, false, errors.Wrap(err, "can't parse fake github server url")
			}

			client.BaseURL = u
			return client, private, nil
		}
		app.GetHooksInjector().AddAfterProviderCreate(func(p provider.Provider) error {
			return p.SetBaseURL(baseURL)
		})
	})
}
