package auth

import (
	"net/http"
	"testing"

	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestGithubLoginFirstTime(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.PUT("/v1/auth/github/unlink").Expect().Status(http.StatusOK)

	// it's guaranteed first time login
	sharedtest.StubLogin(t)
}

func TestGithubLoginNotFirstTime(t *testing.T) {
	sharedtest.StubLogin(t)
	sharedtest.StubLogin(t)
}
