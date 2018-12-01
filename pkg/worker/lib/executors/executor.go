package executors

import "context"

//go:generate mockgen -package executors -source executor.go -destination executor_mock.go

type Executor interface {
	Run(ctx context.Context, name string, args ...string) (string, error)

	WithEnv(k, v string) Executor
	SetEnv(k, v string)

	WorkDir() string
	WithWorkDir(wd string) Executor

	CopyFile(ctx context.Context, dst, src string) error

	Clean()
}
