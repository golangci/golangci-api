package consumers

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
)

type baseConsumer struct {
	eventName           analytics.EventName
	needSendToAnalytics bool
}

const statusOk = "ok"
const statusFail = "fail"

func (c baseConsumer) prepareContext(ctx context.Context, trackingProps map[string]interface{}) context.Context {
	ctx = analytics.ContextWithEventPropsCollector(ctx, c.eventName)
	ctx = analytics.ContextWithTrackingProps(ctx, trackingProps)
	return ctx
}

func (c baseConsumer) wrapConsuming(ctx context.Context, f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v, %s, source is %s", r, debug.Stack(), err)
			analytics.Log(ctx).Errorf("processing of %q task failed: %s", c.eventName, err)
		}
	}()

	analytics.Log(ctx).Infof("Starting consuming of %s...", c.eventName)

	startedAt := time.Now()
	err = f()
	duration := time.Since(startedAt)
	analytics.Log(ctx).Infof("Finished consuming of %s for %s", c.eventName, duration)

	if err != nil {
		analytics.Log(ctx).Errorf("processing of %q task failed: %s", c.eventName, err)
	}

	if c.needSendToAnalytics {
		c.sendAnalytics(ctx, duration, err)
	}

	return err
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
