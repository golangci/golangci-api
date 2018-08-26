package endpointutil

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/apperrors"
	"github.com/golangci/golangci-api/pkg/logutil"
	"github.com/golangci/golangci-api/pkg/request"
)

type contextKey string

const contextKeyRequestContext contextKey = "requestContext"

func RequestContext(ctx context.Context) *request.Context {
	return ctx.Value(contextKeyRequestContext).(*request.Context)
}

func StoreRequestContext(ctx context.Context, rc *request.Context) context.Context {
	return context.WithValue(ctx, contextKeyRequestContext, rc)
}

func MakeRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker) *request.Context {
	lctx := logutil.Context{}
	log = logutil.WrapLogWithContext(log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, et)

	return &request.Context{
		Ctx:       ctx,
		Log:       log,
		Lctx:      lctx,
		StartedAt: time.Now(),
	}
}
