package fetchers

import (
	"context"

	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
)

//go:generate mockgen -package fetchers -source fetcher.go -destination fetcher_mock.go

type Fetcher interface {
	Fetch(ctx context.Context, sg *result.StepGroup, repo *Repo, exec executors.Executor) error
}
