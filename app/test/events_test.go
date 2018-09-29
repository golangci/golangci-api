package test

import (
	"net/http"
	"testing"

	_ "github.com/golangci/golangci-api/app/handlers/auth"
	"github.com/golangci/golangci-api/app/test/sharedtest"
)

func TestPostAnalytitcsEvent(t *testing.T) {
	u := sharedtest.StubLogin(t)
	u.E.POST("/v1/events/analytics").
		WithJSON(map[string]interface{}{
			"name": "test",
			"payload": map[string]interface{}{
				"a": 1,
				"b": "2",
			},
		}).
		Expect().Status(http.StatusOK)
}
