package transportutil

import (
	"net/http"
	"strconv"

	"github.com/golangci/golangci-api/pkg/apierrors"
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
	case apierrors.ErrNotAuthorized:
		return makeError(http.StatusForbidden, e)
	case apierrors.ErrInternal:
		return makeError(http.StatusInternalServerError, errors.New("internal error"))
	}

	return makeError(http.StatusInternalServerError, errors.New("internal error"))
}
