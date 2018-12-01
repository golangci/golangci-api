package analyzequeue

import (
	"fmt"

	"github.com/RichardKnop/machinery/v1/tasks"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/task"
	"github.com/golangci/golangci-api/pkg/worker/lib/queue"
)

func SchedulePRAnalysis(t *task.PRAnalysis) error {
	args := []tasks.Arg{
		{
			Type:  "string",
			Value: t.Repo.Owner,
		},
		{
			Type:  "string",
			Value: t.Repo.Name,
		},
		{
			Type:  "string",
			Value: t.GithubAccessToken,
		},
		{
			Type:  "int",
			Value: t.PullRequestNumber,
		},
		{
			Type:  "string",
			Value: t.APIRequestID,
		},
		{
			Type:  "uint",
			Value: t.UserID,
		},
		{
			Type:  "string",
			Value: t.AnalysisGUID,
		},
	}
	signature := &tasks.Signature{
		Name:         "analyzeV2",
		Args:         args,
		RetryCount:   3,
		RetryTimeout: 600, // 600 sec
	}

	_, err := queue.GetServer().SendTask(signature)
	if err != nil {
		return fmt.Errorf("failed to send the pr analysis task %v to analyze queue: %s", t, err)
	}

	return nil
}

func ScheduleRepoAnalysis(t *task.RepoAnalysis) error {
	args := []tasks.Arg{
		{
			Type:  "string",
			Value: t.Name,
		},
		{
			Type:  "string",
			Value: t.AnalysisGUID,
		},
		{
			Type:  "string",
			Value: t.Branch,
		},
	}
	signature := &tasks.Signature{
		Name:         "analyzeRepo",
		Args:         args,
		RetryCount:   3,
		RetryTimeout: 600, // 600 sec
	}

	_, err := queue.GetServer().SendTask(signature)
	if err != nil {
		return fmt.Errorf("failed to send the repo analysis task %v to analyze queue: %s", t, err)
	}

	return nil
}
