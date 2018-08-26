package apperrors

import (
	"errors"
	"net/http"

	"github.com/stvp/rollbar"
)

type RollbarTracker struct {
	r       *http.Request
	project string
}

func NewRollbarTracker(token, project, env string) *RollbarTracker {
	rollbar.Environment = env
	rollbar.Token = token

	return &RollbarTracker{
		project: project,
	}
}

func (t RollbarTracker) Track(level Level, errorText string, ctx map[string]interface{}) {
	fields := []*rollbar.Field{}

	if ctx != nil {
		fields = append(fields, &rollbar.Field{
			Name: "props",
			Data: ctx,
		})
	}

	fields = append(fields, &rollbar.Field{
		Name: "project",
		Data: t.project,
	})

	var rollbarLevel string
	switch level {
	case LevelError:
		rollbarLevel = rollbar.ERR
	case LevelWarn:
		rollbarLevel = rollbar.WARN
	default:
		panic("invalid level " + level)
	}

	if t.r != nil {
		rollbar.RequestError(rollbarLevel, t.r, errors.New(errorText), fields...)
	} else {
		rollbar.Error(rollbarLevel, errors.New(errorText), fields...)
	}
}

func (t RollbarTracker) WithHTTPRequest(r *http.Request) Tracker {
	t.r = r
	return t
}
