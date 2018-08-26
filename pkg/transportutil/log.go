package transportutil

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/golangci/golangci-shared/pkg/logutil"
)

func AdaptErrorLogger(log logutil.Log) log.Logger {
	return &errorLog{
		sourceLogger: log,
	}
}

type errorLog struct {
	sourceLogger logutil.Log
}

func (el errorLog) Log(values ...interface{}) error {
	s := fmt.Sprint(values...)
	el.sourceLogger.Errorf("gokit transport error: %s", s)
	return nil
}
