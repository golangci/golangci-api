package sharedtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect"
	"github.com/golangci/golangci-api/app/internal/repos"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/satori/go.uuid"
)

type Repo struct {
	returntypes.RepoInfo
	u *User
}

func (u *User) Repos() []Repo {
	initFakeGithubClient()

	respStr := u.E.GET("/v1/repos").
		Expect().
		Status(http.StatusOK).
		Body().
		Raw()

	var reposResp struct {
		Repos []returntypes.RepoInfo
	}
	u.A.NoError(json.Unmarshal([]byte(respStr), &reposResp))
	u.A.Len(reposResp.Repos, 6*2)

	ret := []Repo{}
	for _, r := range reposResp.Repos {
		ret = append(ret, Repo{
			RepoInfo: r,
			u:        u,
		})
	}

	return ret
}

func (u *User) WerePrivateReposFetched() bool {
	initFakeGithubClient()

	respStr := u.E.GET("/v1/repos").
		Expect().
		Status(http.StatusOK).
		Body().
		Raw()

	var resp struct {
		PrivateReposWereFetched bool
	}
	u.A.NoError(json.Unmarshal([]byte(respStr), &resp))

	return resp.PrivateReposWereFetched
}

func (r *Repo) updateFromResponse(resp *httpexpect.Response) {
	respStr := resp.Body().Raw()
	ret := make(map[string]returntypes.RepoInfo)
	r.u.A.NoError(json.Unmarshal([]byte(respStr), &ret))
	r.u.A.NotNil(ret["repo"])
	r.RepoInfo = ret["repo"]
}

func (r *Repo) activateExpectStatus(status int) {
	r.updateFromResponse(
		r.u.E.
			PUT(fmt.Sprintf("/v1/repos/%s", r.Name)).
			Expect().
			Status(status),
	)
}

func (r *Repo) Activate() {
	r.activateExpectStatus(http.StatusOK)
}

func (r *Repo) ActivateFail() {
	r.activateExpectStatus(http.StatusInternalServerError)
}

func (r *Repo) Deactivate() {
	r.updateFromResponse(
		r.u.E.
			DELETE(fmt.Sprintf("/v1/repos/%s", r.Name)).
			Expect().
			Status(http.StatusOK),
	)
}

func (r Repo) ExpectWebhook(eventType string, payload interface{}) *httpexpect.Response {
	// Create new because GitHub makes request without authorization.
	return NewHTTPExpect(r.u.t).
		POST(repos.GetWebhookURLPathForRepo(r.Name, r.HookID)).
		WithJSON(payload).
		WithHeader("X-GitHub-Delivery", uuid.NewV4().String()).
		WithHeader("X-GitHub-Event", eventType).
		Expect()
}

func GetDeactivatedRepo(t *testing.T) (*Repo, *User) {
	u := StubLogin(t)
	r := u.Repos()[0]
	if r.IsActivated {
		r.Deactivate()
	}

	return &r, u
}

func GetActivatedRepo(t *testing.T) (*Repo, *User) {
	u := StubLogin(t)
	r := u.Repos()[0]
	if r.IsActivated {
		return &r, u
	}

	r.Activate()
	return &r, u
}
