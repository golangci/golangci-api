package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/golangci/golangci-shared/pkg/logutil"
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

	Name string
	Args []string
}

type Response struct {
	Error        string
	CommandError string
	StdOut       string
}

const TokenHeaderName = "X-Golangci-Token"

func NewRunner(log logutil.Log, token string) *Runner {
	return &Runner{
		log:   log,
		token: token,
	}
}

func (r Runner) Run(port int, maxLifetime time.Duration) error {
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr}
	r.log.Infof("Listening on %s...", addr)
	go func() {
		time.Sleep(maxLifetime)
		if err := srv.Shutdown(nil); err != nil {
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

		out, err := r.executeRequest(req)
		if err != nil {
			return &Response{
				CommandError: err.Error(),
				StdOut:       out,
			}, nil
		}

		return &Response{StdOut: out}, nil
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

func (r Runner) executeRequest(req *Request) (string, error) {
	timeout := time.Millisecond * time.Duration(req.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, req.Name, req.Args...)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if len(req.Env) != 0 {
		cmd.Env = append(os.Environ(), req.Env...)
	}
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("can't execute command %s %v: err: %s, stdout: %s, stderr: %s",
			req.Name, req.Args, err, string(out), stderrBuf.String())
	}

	return string(out), nil
}
