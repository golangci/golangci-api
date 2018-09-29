package transportutil

import (
	"context"
	"net/http"

	"github.com/golangci/golangci-api/pkg/endpointutil"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/pkg/errors"
)

func FinalizeSession(ctx context.Context, w http.ResponseWriter) context.Context {
	rc := endpointutil.RequestContext(ctx)
	if rc == nil { // was error during request initialization
		return ctx
	}

	authCtx := rc.(*request.AuthorizedContext) // caller must not use this func for not authorized context
	sess := authCtx.Sess
	if err := sess.RunCallbacks(w); err != nil {
		err = errors.Wrap(err, "failed to finalize session")
		authCtx.Log.Errorf("Request failed on session finalization: %s", err)
		return storeContextError(ctx, err)
	}

	return ctx
}
