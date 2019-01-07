package workspaces

import (
	"context"

	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
)

type Installer interface {
	Setup(ctx context.Context, privateAccessToken string, repo *fetchers.Repo, projectPathParts ...string) (executors.Executor, *result.Log, error)
}
