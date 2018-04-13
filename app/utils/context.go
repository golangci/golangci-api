package utils

import (
	cntxt "context"

	"github.com/golangci/golib/server/context"
	"github.com/sirupsen/logrus"
)

func NewBackgroundContext() *context.C {
	return &context.C{
		Ctx: cntxt.Background(),
		L:   logrus.StandardLogger().WithField("ctx", "background"),
	}
}
