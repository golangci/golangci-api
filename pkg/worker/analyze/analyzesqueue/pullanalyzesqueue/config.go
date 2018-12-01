package pullanalyzesqueue

import "github.com/golangci/golangci-api/pkg/worker/lib/github"

const runQueueID = "analyzes/pull/run"

type RunMessage struct {
	github.Context
	APIRequestID string
	UserID       uint
	AnalysisGUID string
}

func (m RunMessage) LockID() string {
	return m.AnalysisGUID
}
