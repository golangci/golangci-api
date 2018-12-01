package logutil

import (
	"fmt"
	"os"

	"github.com/golangci/golangci-lint/pkg/exitcodes"
	"github.com/sirupsen/logrus" //nolint:depguard
)

type StderrLog struct {
	name      string
	logger    *logrus.Logger
	level     LogLevel
	debugKeys map[string]bool
}

var _ Log = NewStderrLog("")

func NewStderrLog(name string, debugKeys ...string) *StderrLog {
	sl := &StderrLog{
		name:      name,
		logger:    logrus.New(),
		level:     LogLevelWarn,
		debugKeys: map[string]bool{},
	}

	for _, k := range debugKeys {
		sl.debugKeys[k] = true
	}

	// control log level in logutils, not in logrus
	sl.logger.SetLevel(logrus.DebugLevel)
	sl.logger.Out = os.Stderr
	sl.logger.Formatter = &logrus.TextFormatter{
		DisableTimestamp: true, // `INFO[0007] msg` -> `INFO msg`
	}
	return sl
}

func (sl StderrLog) prefix() string {
	prefix := ""
	if sl.name != "" {
		prefix = fmt.Sprintf("[%s] ", sl.name)
	}

	return prefix
}

func (sl StderrLog) Fatalf(format string, args ...interface{}) {
	sl.logger.Errorf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
	os.Exit(exitcodes.Failure)
}

func (sl StderrLog) Errorf(format string, args ...interface{}) {
	if sl.level > LogLevelError {
		return
	}

	sl.logger.Errorf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Warnf(format string, args ...interface{}) {
	if sl.level > LogLevelWarn {
		return
	}

	sl.logger.Warnf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Infof(format string, args ...interface{}) {
	if sl.level > LogLevelInfo {
		return
	}

	sl.logger.Infof("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Debugf(key string, format string, args ...interface{}) {
	if sl.level > LogLevelDebug {
		return
	}

	if sl.debugKeys[key] {
		sl.logger.Debugf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
	}
}

func (sl StderrLog) Child(name string) Log {
	prefix := ""
	if sl.name != "" {
		prefix = sl.name + "/"
	}

	child := sl
	child.name = prefix + name

	return &child
}

func (sl *StderrLog) SetLevel(level LogLevel) {
	sl.level = level
}
