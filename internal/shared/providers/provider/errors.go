package provider

import "github.com/pkg/errors"

var (
	ErrUnauthorized    = errors.New("no VCS provider authorization")
	ErrRepoWasArchived = errors.New("repo was archived so is read-only")
	ErrNoFreeOrgSeats  = errors.New("no free seats in GitHub organization") // TODO: remove github
	ErrNotFound        = errors.New("not found in VCS provider")
)

func IsPermanentError(err error) bool {
	causeErr := errors.Cause(err)
	return causeErr == ErrRepoWasArchived || causeErr == ErrNotFound ||
		causeErr == ErrUnauthorized || causeErr == ErrNoFreeOrgSeats
}
