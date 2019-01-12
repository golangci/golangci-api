package processors

import (
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/worker/lib/errorutils"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

var (
	errNothingToAnalyze = errors.New("nothing to analyze")
	errCantAnalyze      = errors.New("can't analyze")
	ErrUnrecoverable    = errors.New("unrecoverable error")
)

type IgnoredError struct {
	Status     github.Status
	StatusDesc string
}

func (e IgnoredError) Error() string {
	return e.StatusDesc
}

// TODO: migrate to golangci-lint linter runner when pr processor will have the same code
func transformError(err error) error {
	if err == nil {
		return nil
	}

	causeErr := errors.Cause(err)
	if causeErr == fetchers.ErrNoBranchOrRepo {
		return errors.Wrap(ErrUnrecoverable, err.Error())
	}

	if ierr, ok := causeErr.(*errorutils.InternalError); ok {
		if strings.Contains(ierr.PrivateDesc, noGoFilesToAnalyzeErr) {
			return errNothingToAnalyze
		}

		return ierr
	}

	if _, ok := causeErr.(*errorutils.BadInputError); ok {
		return errors.Wrap(ErrUnrecoverable, err.Error())
	}

	if _, ok := err.(*IgnoredError); ok {
		return errors.Wrap(ErrUnrecoverable, err.Error())
	}

	return err
}

func isNothingToAnalyzeError(err error) bool {
	err = errors.Cause(err)

	if err == errNothingToAnalyze {
		return true
	}

	if ierr, ok := err.(*errorutils.InternalError); ok {
		if strings.Contains(ierr.PrivateDesc, noGoFilesToAnalyzeErr) {
			return true
		}
	}

	return false
}

func errorToStatus(err error) string {
	err = errors.Cause(err)

	if err == nil || err == ErrUnrecoverable {
		return StatusProcessed
	}

	if isNothingToAnalyzeError(err) {
		return string(StatusProcessed)
	}

	if _, ok := err.(*errorutils.InternalError); ok {
		return string(StatusError)
	}

	if _, ok := err.(*errorutils.BadInputError); ok {
		return string(StatusError)
	}

	if err == fetchers.ErrNoBranchOrRepo {
		return StatusNotFound
	}

	return string(StatusError)
}

func getGithubStatusForIssues(issues []result.Issue) (github.Status, string) {
	switch len(issues) {
	case 0:
		return github.StatusSuccess, "No issues found!"
	case 1:
		return github.StatusFailure, "1 issue found"
	default:
		return github.StatusFailure, fmt.Sprintf("%d issues found", len(issues))
	}
}

func pullErrorToGithubStatusAndDesc(err error, res *result.Result) (github.Status, string) {
	if err == nil {
		return getGithubStatusForIssues(res.Issues)
	}

	err = errors.Cause(err)

	if serr, ok := err.(*IgnoredError); ok {
		return serr.Status, serr.StatusDesc
	}

	if isNothingToAnalyzeError(err) {
		return github.StatusSuccess, noGoFilesToAnalyzeMessage
	}

	if ierr, ok := err.(*errorutils.InternalError); ok {
		return github.StatusError, ierr.PublicDesc
	}

	if _, ok := err.(*errorutils.BadInputError); ok {
		return github.StatusError, errCantAnalyze.Error()
	}

	return github.StatusError, internalError
}

func buildPublicError(err error) string {
	if err == nil {
		return ""
	}

	err = errors.Cause(err)

	if _, ok := err.(*IgnoredError); ok {
		return "" // already must have warning, don't set publicError
	}

	if isNothingToAnalyzeError(err) {
		return errNothingToAnalyze.Error()
	}

	if ierr, ok := err.(*errorutils.InternalError); ok {
		return ierr.PublicDesc
	}

	if _, ok := err.(*errorutils.BadInputError); ok {
		return errCantAnalyze.Error()
	}

	return internalError
}
