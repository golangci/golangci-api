package command

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenv/result"
	"github.com/pkg/errors"
)

type StreamingRunner struct {
	step *result.Step
}

func NewStreamingRunner(step *result.Step) *StreamingRunner {
	return &StreamingRunner{
		step: step,
	}
}

func (r StreamingRunner) wait(ctx context.Context, name string, childPid int, outReader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(outReader)
	lines := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			r.step.AddOutputLine(line)
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "lines scanning error")
	}

	return lines, nil
}

func (r StreamingRunner) Run(ctx context.Context, env []string, name string, args ...string) (string, error) {
	pid, outReader, finish, err := r.runAsync(ctx, env, name, args...)
	if err != nil {
		return "", err
	}

	endCh := make(chan struct{})
	defer close(endCh)

	go func() {
		select {
		case <-ctx.Done():
			_ = outReader.Close()
		case <-endCh:
		}
	}()

	lines, waitErr := r.wait(ctx, name, pid, outReader)

	err = finish()

	if err == nil && waitErr != nil {
		err = waitErr
	}

	// XXX: it's important to not change error here, because it holds exit code
	return strings.Join(lines, "\n"), err
}

type finishFunc func() error

func (r StreamingRunner) runAsync(ctx context.Context, env []string, name string, args ...string) (int, io.ReadCloser, finishFunc, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env

	outReader, err := cmd.StdoutPipe()
	if err != nil {
		return 0, nil, nil, errors.Wrap(err, "can't make out pipe")
	}

	cmd.Stderr = cmd.Stdout // Set the same pipe
	if err := cmd.Start(); err != nil {
		return 0, nil, nil, err
	}

	// XXX: it's important to not change error here, because it holds exit code
	return cmd.Process.Pid, outReader, cmd.Wait, nil
}
