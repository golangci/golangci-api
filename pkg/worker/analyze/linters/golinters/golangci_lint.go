package golinters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/goenvbuild/config"
	logresult "github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/errorutils"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"

	"github.com/golangci/golangci-lint/pkg/printers"
)

type GolangciLint struct {
	PatchPath string
}

func (g GolangciLint) Name() string {
	return "golangci-lint"
}

//nolint:gocyclo
func (g GolangciLint) Run(ctx context.Context, sg *logresult.StepGroup, exec executors.Executor, buildConfig *config.Service) (*result.Result, error) {
	exec = exec.WithEnv("GOLANGCI_COM_RUN", "1")

	analyzedPaths, err := buildConfig.GetValidatedAnalyzedPaths()
	if err != nil {
		sg.AddStepCmd("GOLANGCI_COM_RUN=1 golangci-lint", "run")
		return nil, errors.Wrap(err, "failed to build paths for analysis")
	}

	args := []string{
		"run",
		"--out-format=json",
		"--issues-exit-code=0",
		"--deadline=5m",
		"--new=false",
		"--new-from-rev=",
		"--new-from-patch=" + g.PatchPath,
	}
	args = append(args, analyzedPaths...)
	step := sg.AddStepCmd("GOLANGCI_COM_RUN=1 golangci-lint", args...)

	runRes, runErr := exec.Run(ctx, g.Name(), args...)

	// logrus escapes \n when golangci-lint run not in TTY by user
	stdErr := runRes.StdErr
	stdErr = strings.TrimSpace(stdErr)
	stdErr = strings.TrimPrefix(stdErr, "level=error msg=")
	unquotedStdErr, err := strconv.Unquote(stdErr)
	if err == nil {
		stdErr = unquotedStdErr
	}
	step.AddOutput(stdErr)

	rawJSON := []byte(runRes.StdOut)

	if runErr != nil {
		var res printers.JSONResult
		if jsonErr := json.Unmarshal(rawJSON, &res); jsonErr == nil && res.Report.Error != "" {
			return nil, &errorutils.BadInputError{
				PublicDesc: fmt.Sprintf("can't run golangci-lint: bad input: %s", res.Report.Error),
			}
		}

		// it's not json in the out
		step.AddOutput(runRes.StdOut)

		const badLoadStr = "failed to load program with go/packages"
		if strings.Contains(stdErr, badLoadStr) {
			return nil, &errorutils.BadInputError{
				PublicDesc: badLoadStr,
			}
		}

		return nil, &errorutils.InternalError{
			PublicDesc:  "can't run golangci-lint: internal error",
			PrivateDesc: fmt.Sprintf("can't run golangci-lint: %s", runErr),
			StdErr:      stdErr,
		}
	}

	var res printers.JSONResult
	if jsonErr := json.Unmarshal(rawJSON, &res); jsonErr != nil {
		step.AddOutput(runRes.StdOut)
		return nil, &errorutils.InternalError{
			PublicDesc:  "can't run golangci-lint: invalid output json",
			PrivateDesc: fmt.Sprintf("can't run golangci-lint: can't parse json output: %s", jsonErr),
			StdErr:      stdErr,
		}
	}

	if res.Report != nil && len(res.Report.Warnings) != 0 {
		for _, warn := range res.Report.Warnings {
			step.AddOutputLine("[WARN] %s: %s", warn.Tag, warn.Text)
		}
		analytics.Log(ctx).Infof("Got golangci-lint warnings: %#v", res.Report.Warnings)
	}

	var retIssues []result.Issue
	for _, i := range res.Issues {
		retIssues = append(retIssues, result.Issue{
			File:        i.FilePath(),
			LineNumber:  i.Line(),
			Text:        i.Text,
			FromLinter:  i.FromLinter,
			HunkPos:     i.HunkPos,
			LineRange:   i.LineRange,
			Replacement: i.Replacement,
		})
		step.AddOutputLine("%s:%d: %s (%s)", i.FilePath(), i.Line(), i.Text, i.FromLinter)
	}

	return &result.Result{
		Issues:     retIssues,
		ResultJSON: json.RawMessage(rawJSON),
	}, nil
}
