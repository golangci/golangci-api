package apperrors

import (
	"fmt"

	"github.com/golangci/golangci-api/pkg/logutil"
)

func WrapLogWithTracker(log logutil.Log, lctx logutil.Context, t Tracker) logutil.Log {
	return trackedLog{
		log:  log,
		lctx: lctx,
		t:    t,
	}
}

type trackedLog struct {
	log  logutil.Log
	lctx logutil.Context
	t    Tracker
}

func (tl trackedLog) Fatalf(format string, args ...interface{}) {
	tl.log.Fatalf(format, args...)
}

func (tl trackedLog) Errorf(format string, args ...interface{}) {
	tl.t.Track(LevelError, fmt.Sprintf(format, args...), tl.lctx)
	tl.log.Errorf(format, args...)
}

func (tl trackedLog) Warnf(format string, args ...interface{}) {
	tl.t.Track(LevelWarn, fmt.Sprintf(format, args...), tl.lctx)
	tl.log.Warnf(format, args...)
}

func (tl trackedLog) Infof(format string, args ...interface{}) {
	tl.log.Infof(format, args...)
}

func (tl trackedLog) Debugf(key string, format string, args ...interface{}) {
	tl.log.Debugf(key, format, args...)
}

func (tl trackedLog) Child(name string) logutil.Log {
	return tl.log.Child(name)
}

func (tl trackedLog) SetLevel(level logutil.LogLevel) {
	tl.log.SetLevel(level)
}
