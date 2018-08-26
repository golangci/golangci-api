package transportutil

import (
	"context"
	"net/http"
	"time"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/golangci/golangci-api/pkg/apperrors"
	"github.com/golangci/golangci-api/pkg/endpointutil"
	"github.com/golangci/golangci-api/pkg/logutil"
)

func MakeStoreRequestContext(log logutil.Log, et apperrors.Tracker) httptransport.RequestFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		rc := endpointutil.MakeRequestContext(ctx, log, et.WithHTTPRequest(r))
		return endpointutil.StoreRequestContext(ctx, rc)
	}
}

func FinalizeRequest(ctx context.Context, code int, r *http.Request) {
	rc := endpointutil.RequestContext(ctx)
	rc.Log.Debugf("%s %s respond %d for %s", r.Method, r.URL.Path, code, time.Since(rc.StartedAt))
}
