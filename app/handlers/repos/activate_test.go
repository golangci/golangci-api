package repos

import (
	"net/http"
	"strings"
	"testing"

	_ "github.com/golangci/golangci-api/app/handlers/auth"
	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestActivateNotPut(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.GET("/v1/repos/golangci/repo").Expect().Status(http.StatusNotFound)
}

func TestActivateTeamRepo(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.PUT("/v1/repos/team/somerepo").
		Expect().
		Status(http.StatusInternalServerError) // TODO
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
	r.Activate()
}

func TestDoubleDeactivate(t *testing.T) {
	r, _ := getDeactivatedRepo(t)
	r.Activate()
	r.Deactivate()
	r.Deactivate()
}

func TestActivateWithUpperCase(t *testing.T) {
	r, u := getDeactivatedRepo(t)

	srcName := r.Name
	upperName := strings.ToUpper(srcName)
	u.A.NotEqual(strings.ToLower(srcName), srcName) // to check mapping to activated repos in list of repos
	u.A.NotEqual(upperName, srcName)
	r.Name = upperName

	r.Activate()
	u.A.Equal(upperName, r.Name) // check case was saved
	u.A.True(r.IsActivated)
	u.A.True(u.Repos()[0].IsActivated) // important to check because of mapping

	r.Name = upperName
	r.Deactivate()
	u.A.Equal(upperName, r.Name)
	u.A.False(r.IsActivated)
	u.A.False(u.Repos()[0].IsActivated)
}
