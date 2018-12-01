package transportutil

import (
	"context"
	"net/http"

	"github.com/golangci/golangci-api/internal/api/endpointutil"
	"github.com/pkg/errors"
)

func FinalizeSession(ctx context.Context, w http.ResponseWriter) context.Context {
	rc := endpointutil.RequestContext(ctx)
	if rc == nil { // was error during request initialization
		return ctx
	}

	r := getHTTPRequestFromContext(ctx)
	sessCtx := rc.SessContext()
	if err := sessCtx.Saver.FinalizeHTTP(r, w); err != nil {
		rc.Logger().Errorf("Request failed on session finalization: %s", err)
		err = errors.Wrap(err, "failed to finalize session")
		return storeContextError(ctx, err)
	}

	return ctx
}
