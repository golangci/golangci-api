package logutil

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type Context map[string]interface{}

func WrapLogWithContext(log Log, lctx Context) Log {
	return contextLog{
		lctx: lctx,
		log:  log,
	}
}

type contextLog struct {
	lctx Context
	log  Log
}

func (cl contextLog) wrapFormat(format string) string {
	var pairs []string
	for k, v := range cl.lctx {
		pairs = append(pairs, fmt.Sprintf("%s=%v", color.YellowString(k), v))
	}

	ctx := strings.Join(pairs, " ")
	if ctx != "" {
		ctx = "[" + ctx + "]"
	}
	return fmt.Sprintf("%s %s", format, ctx)
}

func (cl contextLog) Fatalf(format string, args ...interface{}) {
	cl.log.Fatalf(cl.wrapFormat(format), args...)
}

func (cl contextLog) Errorf(format string, args ...interface{}) {
	cl.log.Errorf(cl.wrapFormat(format), args...)
}

func (cl contextLog) Warnf(format string, args ...interface{}) {
	cl.log.Warnf(cl.wrapFormat(format), args...)
}

func (cl contextLog) Infof(format string, args ...interface{}) {
	cl.log.Infof(cl.wrapFormat(format), args...)
}

func (cl contextLog) Debugf(key string, format string, args ...interface{}) {
	cl.log.Debugf(key, cl.wrapFormat(format), args...)
}

func (cl contextLog) Child(name string) Log {
	return cl.log.Child(name)
}

func (cl contextLog) SetLevel(level LogLevel) {
	cl.log.SetLevel(level)
}
