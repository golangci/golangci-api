package linters

import (
	"context"
	"log"

	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
)

type Runner interface {
	Run(ctx context.Context, linters []Linter, exec executors.Executor) (*result.Result, error)
}

type SimpleRunner struct {
}

func (r SimpleRunner) Run(ctx context.Context, linters []Linter, exec executors.Executor) (*result.Result, error) {
	results := []result.Result{}
	for _, linter := range linters {
		res, err := linter.Run(ctx, exec)
		if err != nil {
			return nil, err // don't wrap error here, need to save original error
		}

		results = append(results, *res)
	}

	return r.mergeResults(results), nil
}

func (r SimpleRunner) mergeResults(results []result.Result) *result.Result {
	if len(results) == 0 {
		return nil
	}

	if len(results) > 1 {
		log.Fatalf("len(results) can't be more than 1: %+v", results)
	}

	// TODO: support for multiple linters, not only golangci-lint
	return &results[0]
}
