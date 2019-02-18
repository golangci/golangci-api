package result

import golangciLintResult "github.com/golangci/golangci-lint/pkg/result"

type Issue struct {
	FromLinter string
	Text       string
	File       string
	LineNumber int
	HunkPos    int

	LineRange   *golangciLintResult.Range
	Replacement *golangciLintResult.Replacement
}

func NewIssue(fromLinter, text, file string, lineNumber, hunkPos int) Issue {
	return Issue{
		FromLinter: fromLinter,
		Text:       text,
		File:       file,
		LineNumber: lineNumber,
		HunkPos:    hunkPos,
	}
}
