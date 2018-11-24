package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenv/result"
	"github.com/pkg/errors"
)

type StreamingRunner struct {
	log *result.Log
	wd  string
	env []string
}

func NewStreamingRunner(log *result.Log) *StreamingRunner {
	return &StreamingRunner{
		log: log,
	}
}

func (r StreamingRunner) WithEnv(k, v string) *StreamingRunner {
	return r.WithEnvPair(fmt.Sprintf("%s=%s", k, v))
}

func (r StreamingRunner) WithEnvPair(envPair string) *StreamingRunner {
	return &StreamingRunner{
		log: r.log,
		wd:  r.wd,
		env: append([]string{envPair}, r.env...),
	}
}

func (r StreamingRunner) WithWD(wd string) *StreamingRunner {
	return &StreamingRunner{
		log: r.log,
		wd:  wd,
		env: r.env,
	}
}

func (r StreamingRunner) wait(outReader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(outReader)
	lines := []string{}
	step := r.log.LastStepGroup().LastStep()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			step.AddOutputLine(line)
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Wrap(err, "lines scanning error")
	}

	return lines, nil
}

func (r StreamingRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	_, outReader, finish, err := r.runAsync(ctx, r.env, r.wd, name, args...)
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

	lines, waitErr := r.wait(outReader)

	err = finish()

	if err == nil && waitErr != nil {
		err = waitErr
	}

	// XXX: it's important to not change error here, because it holds exit code
	return strings.Join(lines, "\n"), err
}

type finishFunc func() error

func (r StreamingRunner) runAsync(ctx context.Context, env []string, wd string, name string, args ...string) (int, io.ReadCloser, finishFunc, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Dir = wd

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
