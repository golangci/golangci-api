package errors

import (
	"github.com/golangci/golangci-api/pkg/apperrors"
	"github.com/golangci/golangci-api/pkg/config"
	"github.com/golangci/golangci-api/pkg/logutil"
)

func GetTracker(cfg config.Config, log logutil.Log) apperrors.Tracker {
	env := cfg.GetString("GO_ENV")

	if cfg.GetBool("ROLLBAR_ENABLED", false) {
		return apperrors.NewRollbarTracker(cfg.GetString("ROLLBAR_TOKEN"), "api", env)
	}

	if cfg.GetBool("SENTRY_ENABLED", false) {
		t, err := apperrors.NewSentryTracker(cfg.GetString("SENTRY_DSN"), env)
		if err != nil {
			log.Warnf("Can't make sentry error tracker: %s", err)
			return apperrors.NewNopTracker()
		}

		return t
	}

	return apperrors.NewNopTracker()
}
