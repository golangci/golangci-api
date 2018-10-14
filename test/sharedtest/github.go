package sharedtest

import (
	"sync"

	"github.com/golangci/golangci-api/pkg/app/providers/provider"
)

var initFakeGithubClientOnce sync.Once

func initFakeGithubClient() {
	initFakeGithubClientOnce.Do(func() {
		baseURL := fakeGithubServer.URL + "/"
		app.GetHooksInjector().AddAfterProviderCreate(func(p provider.Provider) error {
			return p.SetBaseURL(baseURL)
		})
	})
}
