package executors

import (
	"context"

	"github.com/pkg/errors"
)

//go:generate mockgen -package executors -source executor.go -destination executor_mock.go

type RunResult struct {
	StdOut string
	StdErr string
}

var ErrExecutorFail = errors.New("executor failed")

type Executor interface {
	Run(ctx context.Context, name string, args ...string) (*RunResult, error)

	WithEnv(k, v string) Executor
	SetEnv(k, v string)

	WorkDir() string
	WithWorkDir(wd string) Executor

	CopyFile(ctx context.Context, dst, src string) error

	Clean()
}
