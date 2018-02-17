package repos

import (
	"net/http"
	"testing"

	_ "github.com/golangci/golangci-api/app/handlers/auth"
	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestActivateNotPut(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.GET("/v1/repos/golangci/repo").Expect().Status(http.StatusNotFound)
}

func TestActivateNotOwnedRepo(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.PUT("/v1/repos/anotherowner/somerepo").
		Expect().
		Status(http.StatusForbidden)
}

func getDeactivatedRepo(t *testing.T) (*sharedtest.Repo, *sharedtest.User) {
	u := sharedtest.StubLogin(t)
	r := u.Repos()[0]
	if r.IsActivated {
		r.Deactivate()
	}

	return &r, u
}

func TestActivate(t *testing.T) {
	r, u := getDeactivatedRepo(t)
	r.Activate()
	u.A.True(u.Repos()[0].IsActivated)
}

func TestDeactivate(t *testing.T) {
	r, u := getDeactivatedRepo(t)
	r.Activate()
	r.Deactivate()
	u.A.False(u.Repos()[0].IsActivated)
}

func TestDoubleActivate(t *testing.T) {
	r, _ := getDeactivatedRepo(t)
	r.Activate()
	//r.ActivateFail()
}
