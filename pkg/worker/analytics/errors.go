package analytics

import (
	"context"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/pkg/worker/lib/runmode"
)

func trackError(ctx context.Context, err error, level apperrors.Level) {
	if !runmode.IsProduction() {
		return
	}

	log := logutil.NewStderrLog("trackError")
	cfg := config.NewEnvConfig(log)
	et := apperrors.GetTracker(cfg, log, "worker")
	et.Track(level, err.Error(), getTrackingProps(ctx))
}
