package test

import (
	"testing"

	"github.com/golangci/golangci-api/test/sharedtest"
)

func TestListRepos(t *testing.T) {
	u := sharedtest.Login(t)
	u.Repos()
}

func TestGithubPrivateLogin(t *testing.T) {
	u := sharedtest.Login(t)
	u.A.True(u.GithubPrivateLogin().WerePrivateReposFetched())
}
