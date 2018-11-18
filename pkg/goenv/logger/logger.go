package logger

import (
	"log"

	"github.com/golangci/golangci-api/pkg/goenv/result"
	"github.com/golangci/golangci-shared/pkg/logutil"
)

type StepGroupLogger struct {
	sg *result.StepGroup
}

func NewStepGroupLogger(sg *result.StepGroup) *StepGroupLogger {
	return &StepGroupLogger{
		sg: sg,
	}
}

func (sgl StepGroupLogger) lastStep() *result.Step {
	return sgl.sg.LastStep()
}

func (sgl StepGroupLogger) Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}
func (sgl StepGroupLogger) Errorf(format string, args ...interface{}) {
	s := sgl.lastStep()
	s.AddOutputLine("[ERROR] "+format, args...)
}

func (sgl StepGroupLogger) Warnf(format string, args ...interface{}) {
	s := sgl.lastStep()
	s.AddOutputLine("[WARN] "+format, args...)
}
func (sgl StepGroupLogger) Infof(format string, args ...interface{}) {
	s := sgl.lastStep()
	s.AddOutputLine(format, args...)
}

func (sgl StepGroupLogger) Debugf(key string, format string, args ...interface{}) {
}

func (sgl StepGroupLogger) Child(name string) logutil.Log {
	panic("child isn't supported")
}

func (sgl StepGroupLogger) SetLevel(level logutil.LogLevel) {
	panic("setlevel isn't supported")
}
