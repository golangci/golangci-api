package apierrors

import (
	"fmt"

	"github.com/pkg/errors"
)

var (
	ErrNotFound      = errors.New("no data")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("interal error")
	ErrNotAuthorized = errors.New("not authorized")
)

type ErrorLikeResult interface {
	IsErrorLikeResult() bool
}

func IsErrorLikeResult(err error) bool {
	err = errors.Cause(err)
	elr, ok := err.(ErrorLikeResult)
	if !ok {
		return false
	}

	return elr.IsErrorLikeResult()
}

type RedirectError struct {
	Temporary bool
	URL       string
}

func (e RedirectError) Error() string {
	return fmt.Sprintf("redirect to %s, temp: %t", e.URL, e.Temporary)
}

func (e RedirectError) IsErrorLikeResult() bool {
	return true
}

func NewTemporaryRedirectError(url string) *RedirectError {
	return &RedirectError{
		Temporary: true,
		URL:       url,
	}
}
