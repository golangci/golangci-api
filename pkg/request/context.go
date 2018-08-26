package request

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/logutil"
)

type Context struct {
	Ctx  context.Context
	Log  logutil.Log
	Lctx logutil.Context

	StartedAt time.Time
}
