package apierrors

import "errors"

var (
	ErrNotFound      = errors.New("no data")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("interal error")
	ErrNotAuthorized = errors.New("not authorized")
)
