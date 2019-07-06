package repoanalyzesqueue

import (
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
)

type Producer struct {
	producers.Base
}

func (p *Producer) Register(m *producers.Multiplexer) error {
	return p.Base.Register(m, runQueueID)
}

func (p Producer) Put(repoName, analysisGUID, branch, privateAccessToken, commitSHA string) error {
	return p.Base.Put(runMessage{
		RepoName:           repoName,
		AnalysisGUID:       analysisGUID,
		Branch:             branch,
		PrivateAccessToken: privateAccessToken,
		CommitSHA:          commitSHA,
	})
}
