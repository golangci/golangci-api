package transportutil

import (
	"context"
	"encoding/json"
	"net/http"
)

func EncodeError(ctx context.Context, err error, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusBadRequest)

	resp := struct {
		Error string
	}{
		Error: err.Error(),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
