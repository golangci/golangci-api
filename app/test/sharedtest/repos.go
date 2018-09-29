package sharedtest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gavv/httpexpect"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/returntypes"
	"github.com/golangci/golangci-api/pkg/todo/repos"
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
	log.Printf("TEST: response: %s", respStr)
	ret := make(map[string]returntypes.RepoInfo)
	r.u.A.NoError(json.Unmarshal([]byte(respStr), &ret))
	r.u.A.NotNil(ret["repo"])
	r.RepoInfo = ret["repo"]
}

func (r *Repo) activateExpectStatus(status int) {
	np := strings.Split(r.Name, "/")
	r.updateFromResponse(
		r.u.E.
			POST("/v1/repos").
			WithJSON(request.BodyRepo{
				Provider: "github.com",
				Owner:    np[0],
				Name:     np[1],
			}).
			Expect().
			Status(status),
	)

	if status != http.StatusOK {
		return
	}

	if r.RepoInfo.IsCreating {
		r.u.A.False(r.RepoInfo.IsDeleting)

		for i := 0; i < 5; i++ {
			r.updateFromResponse(
				r.u.E.
					GET(fmt.Sprintf("/v1/repos/%d", r.RepoInfo.ID)).
					Expect().
					Status(http.StatusOK),
			)
			if !r.RepoInfo.IsCreating {
				break
			}
			time.Sleep(time.Second)
		}
	}
	r.u.A.False(r.RepoInfo.IsCreating)
	r.u.A.False(r.RepoInfo.IsDeleting)
	r.u.A.True(r.RepoInfo.IsActivated)
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
			DELETE(fmt.Sprintf("/v1/repos/%d", r.ID)).
			Expect().
			Status(http.StatusOK),
	)

	if r.RepoInfo.IsDeleting {
		r.u.A.False(r.RepoInfo.IsCreating)

		for i := 0; i < 5; i++ {
			r.updateFromResponse(
				r.u.E.
					GET(fmt.Sprintf("/v1/repos/%d", r.RepoInfo.ID)).
					Expect().
					Status(http.StatusOK),
			)
			if !r.RepoInfo.IsDeleting {
				break
			}
			time.Sleep(time.Second)
		}
	}

	r.u.A.False(r.RepoInfo.IsDeleting)
	r.u.A.False(r.RepoInfo.IsCreating)
	r.u.A.False(r.RepoInfo.IsActivated)
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
	log.Printf("r is %#v", r)
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
