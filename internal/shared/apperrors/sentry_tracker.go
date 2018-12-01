package apperrors

import (
	"fmt"
	"net/http"
	"strings"

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

	errorParts := strings.SplitN(errorText, ": ", 2)
	errorClass := errorParts[0]

	p := raven.NewPacket(errorText, interfaces...)
	p.Fingerprint = []string{errorClass}

	switch level {
	case LevelError:
		p.Level = raven.ERROR
	case LevelWarn:
		p.Level = raven.WARNING
	default:
		panic("invalid level " + level)
	}

	raven.Capture(p, tags)
}

func (t SentryTracker) WithHTTPRequest(r *http.Request) Tracker {
	t.r = r
	return t
}
