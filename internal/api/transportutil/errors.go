package transportutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/pkg/errors"
)

type Error struct {
	HTTPCode int
	Message  string
}

func (e Error) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(e.Message)), nil
}

func (e Error) Error() string {
	return e.Message
}

type ErrorResponse struct {
	Error *Error `json:"error,omitempty"`
}

func makeError(code int, e error) *Error {
	return &Error{
		HTTPCode: code,
		Message:  e.Error(),
	}
}

func MakeError(e error) *Error {
	srcErr := errors.Cause(e)
	switch srcErr {
	case apierrors.ErrNotFound:
		return makeError(http.StatusNotFound, e)
	case apierrors.ErrBadRequest:
		return makeError(http.StatusBadRequest, e)
	case apierrors.ErrNotAuthorized, provider.ErrUnauthorized:
		return makeError(http.StatusForbidden, e)
	case apierrors.ErrInternal:
		return makeError(http.StatusInternalServerError, errors.New("internal error"))
	}

	return makeError(http.StatusInternalServerError, errors.New("internal error"))
}

func HandleErrorLikeResult(ctx context.Context, w http.ResponseWriter, e error) error {
	switch err := e.(type) {
	case *apierrors.RedirectError:
		r := getHTTPRequestFromContext(ctx)
		code := http.StatusPermanentRedirect
		if err.Temporary {
			code = http.StatusTemporaryRedirect
		}
		http.Redirect(w, r, err.URL, code)
		return nil
	case *apierrors.ContinueError:
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusAccepted)
		return errors.Wrapf(json.NewEncoder(w).Encode(err), "while encoding '%s'", err.URL)
	case *apierrors.PendingError:
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusAccepted)
		return nil
	}

	return fmt.Errorf("unknown error like result type: %#v", e)
}
