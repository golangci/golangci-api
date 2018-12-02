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

type LocalizedError interface {
	GetMessage() string
}

type ErrorWithCode interface {
	GetCode() string
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

// ContinueError behaves like RedirectError but instead it's API friendly
// and uses status code 202 with json body.
type ContinueError struct {
	URL string `json:"continueUrl"`
}

func (e ContinueError) Error() string {
	return fmt.Sprintf("continue to %s", e.URL)
}

func (e ContinueError) IsErrorLikeResult() bool {
	return true
}

func NewContinueError(url string) *ContinueError {
	return &ContinueError{
		URL: url,
	}
}

type PendingError struct{}

func (e PendingError) Error() string {
	return fmt.Sprintf("request is still processing")
}

func (e PendingError) IsErrorLikeResult() bool {
	return true
}

type NotAcceptableError struct {
	code    string
	message string
}

func (e NotAcceptableError) Error() string {
	prefix := fmt.Sprintf("not acceptable: %s", e.code)
	if e.message != "" {
		return prefix + ": " + e.message
	}

	return prefix
}

func (e NotAcceptableError) GetMessage() string {
	return e.message
}

func (e NotAcceptableError) GetCode() string {
	return e.code
}

func (e NotAcceptableError) WithMessage(m string) *NotAcceptableError {
	return &NotAcceptableError{
		code:    e.code,
		message: m,
	}
}

func NewNotAcceptableError(code string) *NotAcceptableError {
	return &NotAcceptableError{code: code}
}

type RaceConditionError struct {
	message string
}

func NewRaceConditionError(m string) *RaceConditionError {
	return &RaceConditionError{message: m}
}

func (e RaceConditionError) Error() string {
	return fmt.Sprintf("race condition: %s", e.message)
}

func (e RaceConditionError) GetMessage() string {
	return e.message
}
