package consumers

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/analytics"
)

type baseConsumer struct {
	eventName analytics.EventName
	cfg       config.Config
}

const statusOk = "ok"
const statusFail = "fail"

func (c baseConsumer) prepareContext(ctx context.Context, trackingProps map[string]interface{}) context.Context {
	ctx = analytics.ContextWithEventPropsCollector(ctx, c.eventName)
	ctx = analytics.ContextWithTrackingProps(ctx, trackingProps)
	return ctx
}

func (c baseConsumer) getUnrecoverableErrorLogger(log logutil.Log, repoFullName string) logutil.Func {
	ignoredRepos := c.cfg.GetStringList("LOG_AS_INFO_UNRECOVERABLE_ERRORS_FOR_REPOS")
	for _, ignoredRepo := range ignoredRepos {
		if strings.EqualFold(ignoredRepo, repoFullName) {
			return log.Infof
		}
	}

	return log.Warnf
}

func (c baseConsumer) getRecoverableErrorLogger(log logutil.Log, repoFullName string) logutil.Func {
	ignoredRepos := c.cfg.GetStringList("LOG_AS_INFO_RECOVERABLE_ERRORS_FOR_REPOS")
	for _, ignoredRepo := range ignoredRepos {
		if strings.EqualFold(ignoredRepo, repoFullName) {
			return log.Infof
		}
	}

	return log.Errorf
}

func (c baseConsumer) wrapConsuming(ctx context.Context, log logutil.Log, repoFullName string, f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// no errors.Wrap: err may be nil
			err = fmt.Errorf("%s: panic recovered: %v, %s", err, r, debug.Stack())
			log.Errorf("Processing of %q task failed: %s", c.eventName, err)
		}
	}()

	log.Infof("Starting consuming of %s...", c.eventName)

	startedAt := time.Now()
	err = f()
	duration := time.Since(startedAt)
	log.Infof("Finished consuming of %s for %s", c.eventName, duration)

	if c.eventName == analytics.EventPRChecked {
		c.sendAnalytics(ctx, duration, err)
	}

	if err != nil {
		if isRecoverableError(err) {
			logger := c.getRecoverableErrorLogger(log, repoFullName)
			logger("Processing of %q task failed, retry: %s", c.eventName, err)
			return err
		}

		logger := c.getUnrecoverableErrorLogger(log, repoFullName)
		logger("Processing of %q task failed, error isn't recoverable, delete the task: %s", c.eventName, err)
		return nil
	}

	log.Infof("Processing of %q task succeeded", c.eventName)
	return nil
}

func (c baseConsumer) sendAnalytics(ctx context.Context, duration time.Duration, err error) {
	props := map[string]interface{}{
		"durationSeconds": int(duration / time.Second),
	}
	if err == nil {
		props["status"] = statusOk
	} else {
		props["status"] = statusFail
		props["error"] = err.Error()
	}
	analytics.SaveEventProps(ctx, c.eventName, props)

	tracker := analytics.GetTracker(ctx)
	tracker.Track(ctx, c.eventName)
}
