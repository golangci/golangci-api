package sharedtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/stretchr/testify/assert"
)

type User struct {
	returntypes.AuthorizedUser
	A *assert.Assertions
	E *httpexpect.Expect
	t *testing.T
}

func NewHTTPExpect(t *testing.T) *httpexpect.Expect {
	return httpexpect.New(t, server.URL)
}

func StubLogin(t *testing.T) *User {
	initEnv()
	initServer()

	e := NewHTTPExpect(t)
	fakeGithubServerOnce.Do(initFakeGithubServer)

	e.GET("/v1/auth/check").
		Expect().
		Status(http.StatusForbidden)
	e.GET("/v1/auth/github").
		Expect().
		Status(http.StatusNotFound) // WEB_ROOT
	checkBody := e.GET("/v1/auth/check").
		Expect().
		Status(http.StatusOK).
		Body().
		Raw()

	userResp := make(map[string]*User)
	assert.NoError(t, json.Unmarshal([]byte(checkBody), &userResp))
	assert.NotNil(t, userResp["user"])
	user := userResp["user"]
	assert.NotZero(t, user.ID)

	user.A = assert.New(t)
	user.E = e
	user.t = t
	return user
}
