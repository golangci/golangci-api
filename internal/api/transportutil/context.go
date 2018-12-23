package transportutil

import (
	"context"
	"net/http"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/golangci/golangci-api/internal/api/endpointutil"
	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

func makeSessionContext(r *http.Request, log logutil.Log) *session.RequestContext {
	return &session.RequestContext{
		Saver:    session.NewSaver(log),
		Registry: sessions.GetRegistry(r),
	}
}

func MakeStoreInternalRequestContext(hctx endpointutil.HandlerRegContext) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		hctx.ErrTracker = hctx.ErrTracker.WithHTTPRequest(r)

		rc, err := endpointutil.MakeInternalRequestContext(ctx,
			makeSessionContext(r, hctx.Log), &hctx,
			r.Header.Get("X-Internal-Access-Token"))
		if err != nil {
			return endpointutil.StoreError(ctx, errors.Wrap(err, "failed to authorize internal request"))
		}

		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func MakeStoreAnonymousRequestContext(hctx endpointutil.HandlerRegContext) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		hctx.ErrTracker = hctx.ErrTracker.WithHTTPRequest(r)
		rc := endpointutil.MakeAnonymousRequestContext(ctx, makeSessionContext(r, hctx.Log), &hctx)
		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func MakeStoreAuthorizedRequestContext(hctx endpointutil.HandlerRegContext) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		hctx.ErrTracker = hctx.ErrTracker.WithHTTPRequest(r)
		rc, err := endpointutil.MakeAuthorizedRequestContext(ctx, makeSessionContext(r, hctx.Log), &hctx)
		if err != nil {
			return endpointutil.StoreError(ctx, errors.Wrap(err, "failed to authorize"))
		}

		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func FinalizeRequest(ctx context.Context, code int, r *http.Request) {
	rc := endpointutil.RequestContext(ctx)
	if rc != nil {
		rc.Logger().Debugf("%s %s respond %d for %s", r.Method, r.URL.Path, code, time.Since(rc.RequestStartedAt()))
	} else {
		logger := logutil.NewStderrLog("finalize request")
		logger.Debugf("%s %s respond %d with no request context", r.Method, r.URL.Path, code)
	}
}

type ctxKey string

const (
	errKey         ctxKey = "transport/error"
	httpRequestKey ctxKey = "transport/httpRequest"
)

func storeContextError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, errKey, err)
}

func GetContextError(ctx context.Context) error {
	v := ctx.Value(errKey)
	if v == nil {
		return nil
	}

	return v.(error)
}

func StoreHTTPRequestToContext(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey, r)
}

func getHTTPRequestFromContext(ctx context.Context) *http.Request {
	return ctx.Value(httpRequestKey).(*http.Request)
}
