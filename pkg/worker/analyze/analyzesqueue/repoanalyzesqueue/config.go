package repoanalyzesqueue

const runQueueID = "analyzes/repo/run"

type runMessage struct {
	RepoName     string
	AnalysisGUID string
	Branch       string
}

func (m runMessage) LockID() string {
	return m.AnalysisGUID
}
