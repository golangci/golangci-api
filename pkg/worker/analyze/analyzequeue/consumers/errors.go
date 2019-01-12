package consumers

import (
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/pkg/errors"
)

func isRecoverableError(err error) bool {
	err = errors.Cause(err)
	if err == processors.ErrUnrecoverable {
		return false
	}

	return github.IsRecoverableError(err)
}
