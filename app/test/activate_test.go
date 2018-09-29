package test

import (
	"testing"

	_ "github.com/golangci/golangci-api/app/handlers/auth"
	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestActivate(t *testing.T) {
	r, u := sharedtest.GetDeactivatedRepo(t)
	r.Activate()
	u.A.True(u.Repos()[0].IsActivated)
}

func TestDeactivate(t *testing.T) {
	r, u := sharedtest.GetDeactivatedRepo(t)
	r.Activate()
	r.Deactivate()
	u.A.False(u.Repos()[0].IsActivated)
}

func TestDoubleActivate(t *testing.T) {
	r, _ := sharedtest.GetDeactivatedRepo(t)
	r.Activate()
	r.Activate()
}

func TestDoubleDeactivate(t *testing.T) {
	r, _ := sharedtest.GetDeactivatedRepo(t)
	r.Activate()
	r.Deactivate()
	r.Deactivate()
}
