package transportutil

import (
	"context"
	"net/http"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/golangci/golangci-api/pkg/endpointutil"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/apperrors"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func MakeStoreAnonymousRequestContext(log logutil.Log, et apperrors.Tracker, db *gorm.DB) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rc := endpointutil.MakeAnonymousRequestContext(ctx, log, et.WithHTTPRequest(r), db)
		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func MakeStoreAuthorizedRequestContext(log logutil.Log, et apperrors.Tracker, db *gorm.DB, sf *session.Factory) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		sess, err := sf.Build(r)
		if err != nil {
			return endpointutil.StoreError(ctx, errors.Wrap(err, "failed to build session"))
		}

		rc, err := endpointutil.MakeAuthorizedRequestContext(ctx, log, et.WithHTTPRequest(r), db, sess)
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

const errKey ctxKey = "transport/error"

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
