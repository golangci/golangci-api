package reporters

import (
	"context"

	envbuildresult "github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
)

//go:generate mockgen -package reporters -source reporter.go -destination reporter_mock.go

type Reporter interface {
	Report(ctx context.Context, buildLog *envbuildresult.Log, ref string, issues []result.Issue) error
}
