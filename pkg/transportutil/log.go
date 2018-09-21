package transportutil

import (
	"fmt"
	"strings"

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
	parts := []string{}
	for _, v := range values {
		parts = append(parts, fmt.Sprint(v))
	}
	el.sourceLogger.Errorf("gokit transport error: %s", strings.Join(parts, ","))
	return nil
}
