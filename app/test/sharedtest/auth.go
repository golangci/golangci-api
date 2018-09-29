package sharedtest

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gavv/httpexpect"
	"github.com/golangci/golangci-api/pkg/returntypes"
	"github.com/stretchr/testify/require"
)

type User struct {
	returntypes.AuthorizedUser
	A *require.Assertions
	E *httpexpect.Expect
	t *testing.T
}

func NewHTTPExpect(t *testing.T) *httpexpect.Expect {
	httpClient := &http.Client{
		Jar: httpexpect.NewJar(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			isRedirectToFakeGithub := strings.HasPrefix(req.URL.String(), fakeGithubServer.URL)
			if isRedirectToFakeGithub || strings.HasPrefix(req.URL.Path, "/v1/auth/github") {
				return nil // follow redirect
			}

			return http.ErrUseLastResponse // don't follow redirect: it's redirect after successful login
		},
	}

	return httpexpect.WithConfig(httpexpect.Config{
		BaseURL:  server.URL,
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{
			httpexpect.NewCompactPrinter(t),
		},
		Client: httpClient,
	})
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
		Status(http.StatusTemporaryRedirect).
		Header("Location").
		Equal(os.Getenv("WEB_ROOT") + "/repos/github?after=login")

	checkBody := e.GET("/v1/auth/check").
		Expect().
		Status(http.StatusOK).
		Body().
		Raw()

	userResp := make(map[string]*User)
	require.NoError(t, json.Unmarshal([]byte(checkBody), &userResp))
	user := userResp["user"]
	require.NotNil(t, user)
	require.NotZero(t, user.ID)

	user.A = require.New(t)
	user.E = e
	user.t = t
	return user
}

func (u *User) GithubPrivateLogin() *User {
	u.E.GET("/v1/auth/github/private").
		Expect().
		Status(http.StatusTemporaryRedirect).
		Header("Location").
		Equal(os.Getenv("WEB_ROOT") + "/repos/github?refresh=1&after=private_login")
	return u
}
