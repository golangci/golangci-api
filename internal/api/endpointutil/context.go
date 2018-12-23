package endpointutil

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/internal/api/apierrors"

	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

type contextKey string

const (
	contextKeyRequestContext contextKey = "endpoint/requestContext"
	contextKeyError          contextKey = "endpoint/error"
)

func RequestContext(ctx context.Context) request.Context {
	rc := ctx.Value(contextKeyRequestContext)
	if rc == nil {
		return nil
	}
	return rc.(request.Context)
}

func StoreRequestContext(ctx context.Context, rc request.Context) context.Context {
	return context.WithValue(ctx, contextKeyRequestContext, rc)
}

func StoreError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, contextKeyError, err)
}

func Error(ctx context.Context) error {
	v := ctx.Value(contextKeyError)
	if v == nil {
		return nil
	}

	return v.(error)
}

func makeBaseRequestContext(ctx context.Context, sctx *session.RequestContext, hctx *HandlerRegContext) *request.BaseContext {
	lctx := logutil.Context{}
	log := hctx.Log
	log = logutil.WrapLogWithContext(log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, hctx.ErrTracker)

	return &request.BaseContext{
		Ctx:       ctx,
		Log:       log,
		Lctx:      lctx,
		DB:        hctx.DB,
		StartedAt: time.Now(),
		SessCtx:   sctx,
	}
}

func MakeAnonymousRequestContext(ctx context.Context, sctx *session.RequestContext, hctx *HandlerRegContext) *request.AnonymousContext {
	return &request.AnonymousContext{
		BaseContext: *makeBaseRequestContext(ctx, sctx, hctx),
	}
}

func MakeInternalRequestContext(ctx context.Context, sctx *session.RequestContext, hctx *HandlerRegContext,
	requestAccessToken string) (*request.InternalContext, error) {

	validAccessToken := hctx.Cfg.GetString("INTERNAL_ACCESS_TOKEN")
	if len(validAccessToken) <= 8 {
		return nil, errors.Wrap(apierrors.ErrNotAuthorized, "too short INTERNAL_ACCESS_TOKEN")
	}

	if validAccessToken != requestAccessToken {
		hctx.Log.Warnf("Invalid internal request access token %q, must be %q",
			requestAccessToken, validAccessToken)
		return nil, errors.Wrap(apierrors.ErrNotAuthorized, "invalid internal access token")
	}

	return &request.InternalContext{
		BaseContext: *makeBaseRequestContext(ctx, sctx, hctx),
	}, nil
}

func MakeAuthorizedRequestContext(ctx context.Context, sctx *session.RequestContext,
	hctx *HandlerRegContext) (*request.AuthorizedContext, error) {

	au, err := hctx.Authorizer.Authorize(sctx)
	if err != nil {
		return nil, err
	}

	baseCtx := makeBaseRequestContext(ctx, sctx, hctx)
	baseCtx.Lctx["user_id"] = au.User.ID
	baseCtx.Lctx["email"] = au.User.Email
	baseCtx.Lctx["provider_login"] = au.Auth.Login

	return &request.AuthorizedContext{
		BaseContext:       *baseCtx,
		AuthenticatedUser: *au,
	}, nil
}
