package events

import (
	"encoding/json"
	"net/http"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	sharedevents "github.com/golangci/golangci-api/pkg/todo/events"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
)

type analyticsEvent struct {
	Name    string
	Payload map[string]interface{}
}

func postAnalyticsEvent(ctx context.C) error {
	if ctx.R.Method != http.MethodPost {
		return herrors.New400Errorf("bad method %q", ctx.R.Method)
	}

	ga, err := user.GetAuth(&ctx)
	if err != nil {
		return herrors.New(err, "can't get current auth")
	}

	var ev analyticsEvent
	if err := json.NewDecoder(ctx.R.Body).Decode(&ev); err != nil {
		return herrors.New400Errorf("invalid request json: %s", err)
	}

	sharedevents.NewAuthenticatedTracker(int(ga.UserID)).Track(ctx.Ctx, ev.Name, ev.Payload)
	return nil
}

func init() {
	handlers.Register("/v1/events/analytics", postAnalyticsEvent)
}
