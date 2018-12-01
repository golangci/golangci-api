package reporters

import (
	"context"

	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
)

//go:generate mockgen -package reporters -source reporter.go -destination reporter_mock.go

type Reporter interface {
	Report(ctx context.Context, ref string, issues []result.Issue) error
}
