package test

import (
	"testing"

	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestListRepos(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.Repos()
}

func TestGithubPrivateLogin(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.A.True(u.GithubPrivateLogin().WerePrivateReposFetched())
}
