package apperrors

import (
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
)

func GetTracker(cfg config.Config, log logutil.Log, project string) Tracker {
	env := cfg.GetString("GO_ENV")

	if cfg.GetBool("ROLLBAR_ENABLED", false) {
		return NewRollbarTracker(cfg.GetString("ROLLBAR_TOKEN"), project, env)
	}

	if cfg.GetBool("SENTRY_ENABLED", false) {
		t, err := NewSentryTracker(cfg.GetString("SENTRY_DSN"), env)
		if err != nil {
			log.Warnf("Can't make sentry error tracker: %s", err)
			return NewNopTracker()
		}

		return t
	}

	return NewNopTracker()
}
