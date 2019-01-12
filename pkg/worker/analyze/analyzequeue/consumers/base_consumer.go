package consumers

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/analytics"
)

type baseConsumer struct {
	eventName analytics.EventName
}

const statusOk = "ok"
const statusFail = "fail"

func (c baseConsumer) prepareContext(ctx context.Context, trackingProps map[string]interface{}) context.Context {
	ctx = analytics.ContextWithEventPropsCollector(ctx, c.eventName)
	ctx = analytics.ContextWithTrackingProps(ctx, trackingProps)
	return ctx
}

func (c baseConsumer) wrapConsuming(log logutil.Log, f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: panic recovered: %v, %s", err, r, debug.Stack())
			log.Errorf("Processing of %q task failed: %s", c.eventName, err)
		}
	}()

	log.Infof("Starting consuming of %s...", c.eventName)

	startedAt := time.Now()
	err = f()
	duration := time.Since(startedAt)
	log.Infof("Finished consuming of %s for %s", c.eventName, duration)

	if err != nil {
		log.Errorf("Processing of %q task failed: %s", c.eventName, err)
	}

	return err
}
