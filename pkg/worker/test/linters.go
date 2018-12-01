package test

import (
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
)

func NewIssue(linter, message string, line int) result.Issue {
	return result.Issue{
		FromLinter: linter,
		Text:       message,
		File:       "p/f.go",
		LineNumber: line,
	}
}
