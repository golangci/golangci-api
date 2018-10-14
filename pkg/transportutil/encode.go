package transportutil

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/golangci/golangci-api/pkg/app/returntypes"
)

func EncodeError(ctx context.Context, err error, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusBadRequest)

	resp := returntypes.Error{
		Error: err.Error(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
