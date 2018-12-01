package task

import "github.com/golangci/golangci-api/pkg/worker/lib/github"

type PRAnalysis struct {
	github.Context
	APIRequestID string
	UserID       uint
	AnalysisGUID string
}

type RepoAnalysis struct {
	Name         string
	AnalysisGUID string
	Branch       string
}
