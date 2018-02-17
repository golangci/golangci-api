package repos

import (
	"testing"

	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestListRepos(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.Repos()
}
