package logger

import (
	"log"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
)

type BuildLogger struct {
	buildLog *result.Log
	logger   logutil.Log
}

func NewBuildLogger(buildLog *result.Log, logger logutil.Log) *BuildLogger {
	return &BuildLogger{
		buildLog: buildLog,
		logger:   logger,
	}
}

func (bl BuildLogger) lastStep() *result.Step {
	return bl.buildLog.LastStepGroup().LastStep()
}

func (bl BuildLogger) Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}
func (bl BuildLogger) Errorf(format string, args ...interface{}) {
	s := bl.lastStep()
	s.AddOutputLine("[ERROR] "+format, args...)
	bl.logger.Errorf(format, args...)
}

func (bl BuildLogger) Warnf(format string, args ...interface{}) {
	s := bl.lastStep()
	s.AddOutputLine("[WARN] "+format, args...)
	bl.logger.Warnf(format, args...)
}
func (bl BuildLogger) Infof(format string, args ...interface{}) {
	s := bl.lastStep()
	s.AddOutputLine(format, args...)
}

func (bl BuildLogger) Debugf(key string, format string, args ...interface{}) {
}

func (bl BuildLogger) Child(name string) logutil.Log {
	panic("child isn't supported")
}

func (bl BuildLogger) SetLevel(level logutil.LogLevel) {
	panic("setlevel isn't supported")
}
