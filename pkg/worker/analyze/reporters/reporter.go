package reporters

import (
	"context"

	"github.com/golangci/golangci-api/pkg/goenvbuild/config"

	envbuildresult "github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
)

//go:generate mockgen -package reporters -source reporter.go -destination reporter_mock.go

type Reporter interface {
	Report(ctx context.Context, buildConfig *config.Service, buildLog *envbuildresult.Log, ref string, issues []result.Issue) error
}
