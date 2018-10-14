package transportutil

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/sessions"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/golangci/golangci-api/pkg/endpoint/endpointutil"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/apperrors"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func makeSessionContext(r *http.Request, log logutil.Log) *session.RequestContext {
	return &session.RequestContext{
		Saver:    session.NewSaver(log),
		Registry: sessions.GetRegistry(r),
	}
}

func MakeStoreAnonymousRequestContext(log logutil.Log, et apperrors.Tracker, db *gorm.DB) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rc := endpointutil.MakeAnonymousRequestContext(ctx, log, et.WithHTTPRequest(r),
			db, makeSessionContext(r, log))
		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func MakeStoreAuthorizedRequestContext(log logutil.Log, et apperrors.Tracker, db *gorm.DB, sf *session.Factory) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rc, err := endpointutil.MakeAuthorizedRequestContext(ctx, log, et.WithHTTPRequest(r),
			db, sf, makeSessionContext(r, log))
		if err != nil {
			return endpointutil.StoreError(ctx, errors.Wrap(err, "failed to make authorized request context"))
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
