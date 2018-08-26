package apperrors

import (
	"fmt"
	"net/http"

	"github.com/getsentry/raven-go"
	"github.com/pkg/errors"
)

type SentryTracker struct {
	r *http.Request
}

func NewSentryTracker(dsn, env string) (*SentryTracker, error) {
	raven.SetEnvironment(env)
	err := raven.SetDSN(dsn)
	if err != nil {
		return nil, errors.Wrap(err, "can't set sentry dsn")
	}

	return &SentryTracker{}, nil
}

func (t SentryTracker) Track(level Level, errorText string, ctx map[string]interface{}) {
	tags := map[string]string{}
	for k, v := range ctx {
		tags[k] = fmt.Sprintf("%v", v)
	}

	var interfaces []raven.Interface
	if t.r != nil {
		interfaces = append(interfaces, raven.NewHttp(t.r))
	}

	switch level {
	case LevelError:
		raven.CaptureError(errors.New(errorText), tags, interfaces...)
	case LevelWarn:
		raven.CaptureMessage(errorText, tags, interfaces...)
	default:
		panic("invalid level " + level)
	}
}

func (t SentryTracker) WithHTTPRequest(r *http.Request) Tracker {
	t.r = r
	return t
}
