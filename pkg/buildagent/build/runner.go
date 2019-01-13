package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/pkg/errors"
)

type Runner struct {
	log   logutil.Log
	token string
}

type Request struct {
	TimeoutMs uint

	WorkDir string
	Env     []string

	Kind string
	Args []string
}

type Response struct {
	Error        string
	CommandError string

	RequestResult
}

const (
	TokenHeaderName = "X-Golangci-Token"

	RequestKindRun  = "run"
	RequestKindCopy = "copy"
)

func NewRunner(log logutil.Log, token string) *Runner {
	return &Runner{
		log:   log,
		token: token,
	}
}

func (r Runner) Run(port int, maxLifetime time.Duration) error {
	if r.token == "" {
		return errors.New("no token")
	}

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr}
	r.log.Infof("Listening on %s...", addr)
	go func() {
		time.Sleep(maxLifetime)
		if err := srv.Shutdown(context.TODO()); err != nil {
			r.log.Warnf("HTTP server shutdown failed: %s", err)
		} else {
			r.log.Infof("HTTP server shutdown succeeded")
		}
	}()

	http.HandleFunc("/", r.handleRequest)
	return srv.ListenAndServe()
}

func (r Runner) handleRequest(w http.ResponseWriter, hr *http.Request) {
	startedAt := time.Now()

	var req *Request
	resp, err := func() (*Response, error) {
		var err error
		req, err = r.parseRequest(hr)
		if err != nil {
			return nil, err
		}

		token := hr.Header.Get(TokenHeaderName)
		if token != r.token {
			r.log.Warnf("Invalid token in request: %q (request) != %q (reference)", token, r.token)
			return nil, errors.New("invalid request token")
		}

		res, err := r.executeRequest(req)
		var commandError string
		if err != nil {
			commandError = err.Error()
		}

		return &Response{
			CommandError:  commandError,
			RequestResult: *res,
		}, nil
	}()

	if err != nil {
		resp = &Response{
			Error: err.Error(),
		}
		r.log.Errorf("Respond with error %#v for request %#v for %s", resp, req, time.Since(startedAt))
	} else {
		r.log.Infof("Respond ok %#v for request %#v for %s", resp, req, time.Since(startedAt))
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		r.log.Errorf("Failed to json encode response %#v: %s", resp, err)
	}
}

func (r Runner) parseRequest(hr *http.Request) (*Request, error) {
	if hr.Method != http.MethodPost {
		return nil, fmt.Errorf("invalid http method %s, expected POST", hr.Method)
	}

	if hr.Body == nil {
		return nil, fmt.Errorf("no http request body")
	}

	var req Request
	if err := json.NewDecoder(hr.Body).Decode(&req); err != nil {
		return nil, errors.Wrap(err, "failed to decode http request body as json to request")
	}

	return &req, nil
}

type RequestResult struct {
	StdOut string
	StdErr string
}

func (r Runner) executeRequest(req *Request) (*RequestResult, error) {
	switch req.Kind {
	case RequestKindRun:
		return r.executeRunRequest(req)
	case RequestKindCopy:
		return r.executeCopyRequest(req)
	}

	return nil, fmt.Errorf("invalid request kind %s", req.Kind)
}

func (r Runner) executeRunRequest(req *Request) (*RequestResult, error) {
	timeout := time.Millisecond * time.Duration(req.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if len(req.Args) == 0 {
		return nil, errors.New("empty args")
	}
	var args []string
	if len(req.Args) > 1 {
		args = req.Args[1:]
	}

	cmd := exec.CommandContext(ctx, req.Args[0], args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if len(req.Env) != 0 {
		cmd.Env = append(os.Environ(), req.Env...)
	}
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	out, err := cmd.Output()
	rr := &RequestResult{
		StdOut: string(out),
		StdErr: stderrBuf.String(),
	}
	if err != nil {
		return rr, err
	}

	if rr.StdErr != "" {
		r.log.Warnf("Command %v stdout: %d, stderr: %s", req.Args, rr.StdOut, rr.StdErr)
	}

	return rr, nil
}

func (r Runner) executeCopyRequest(req *Request) (*RequestResult, error) {
	if len(req.Args) != 2 {
		return nil, fmt.Errorf("invalid args count: %d != 2", len(req.Args))
	}

	destFile := req.Args[0]
	if req.WorkDir != "" {
		destFile = filepath.Join(req.WorkDir, destFile)
	}
	fileContent := req.Args[1]

	if err := ioutil.WriteFile(destFile, []byte(fileContent), os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "failed to write to file %s", destFile)
	}

	return &RequestResult{}, nil
}
