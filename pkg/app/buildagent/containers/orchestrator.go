package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/app/buildagent/build"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

type Orchestrator struct {
	log   logutil.Log
	token string
}

const TokenHeaderName = "X-Golangci-Token"
const minDockerPort = 7001
const maxDockerPort = 8000

type SetupContainerRequest struct {
	TimeoutMs uint
}

type errorResponse struct {
	Error string `json:"omitempty"`
}

type ContainerID struct {
	ID         string
	Port       int
	BuildToken string
}

type SetupContainerResponse struct {
	ContainerID
}

type BuildCommandRequest struct {
	ContainerID
	build.Request
}

type BuildCommandResponse struct {
	BuildResponse build.Response
}

type ShutdownContainerRequest struct {
	TimeoutMs uint
	ContainerID
}

type ShutdownContainerResponse errorResponse

func NewOrchestrator(log logutil.Log, token string) *Orchestrator {
	return &Orchestrator{
		log:   log,
		token: token,
	}
}

func (o Orchestrator) Run(port int) error {
	if o.token == "" {
		return errors.New("no token")
	}

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{Addr: addr}
	o.log.Infof("Listening on %s...", addr)

	http.HandleFunc("/setup", o.handleSetupRequest)
	http.HandleFunc("/shutdown", o.handleShutdownRequest)
	http.HandleFunc("/buildcommand", o.handleBuildCommandRequest)
	return srv.ListenAndServe()
}

func (o Orchestrator) wrapHTTPHandler(w io.Writer, hr *http.Request,
	h func(hr *http.Request) (interface{}, error)) {

	startedAt := time.Now()
	resp, err := func() (interface{}, error) {
		if hr.Method != http.MethodPost {
			return nil, fmt.Errorf("invalid http method %s, expected POST", hr.Method)
		}

		if hr.Body == nil {
			return nil, fmt.Errorf("no http request body")
		}

		token := hr.Header.Get(TokenHeaderName)
		if token != o.token {
			o.log.Warnf("Invalid token in request: %q (request) != %q (reference)", token, o.token)
			return nil, errors.New("invalid request token")
		}

		return h(hr)
	}()

	if err != nil {
		resp = &errorResponse{
			Error: err.Error(),
		}
		o.log.Errorf("[%s] Respond with error %#v for %s", hr.URL.RequestURI(), resp, time.Since(startedAt))
	} else {
		o.log.Infof("[%s] Respond ok %#v for %s", hr.URL.RequestURI(), resp, time.Since(startedAt))
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		o.log.Errorf("Failed to json encode response %#v: %s", resp, err)
	}
}

func (o Orchestrator) parseRequest(hr *http.Request, req interface{}) error {
	if err := json.NewDecoder(hr.Body).Decode(req); err != nil {
		return errors.Wrap(err, "failed to decode http request body as json to request")
	}

	o.log.Infof("Parsed request %#v", req)
	return nil
}

func (o Orchestrator) handleSetupRequest(w http.ResponseWriter, hr *http.Request) {
	o.wrapHTTPHandler(w, hr, func(hr *http.Request) (interface{}, error) {
		var req SetupContainerRequest
		if err := o.parseRequest(hr, &req); err != nil {
			return nil, err
		}

		return o.setupContainer(&req)
	})
}

func genRandomDockerPort() int {
	p := rand.Int()
	return minDockerPort + p%(maxDockerPort-minDockerPort+1)
}

func (o Orchestrator) setupContainer(req *SetupContainerRequest) (*SetupContainerResponse, error) {
	timeout := time.Millisecond * time.Duration(req.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	buildToken := uuid.NewV4().String()

	attemptsN := 3
	for i := 0; i < attemptsN; i++ {
		port := genRandomDockerPort()
		const runnerPort = 7000
		portsMapping := fmt.Sprintf("127.0.0.1:%d:%d", port, runnerPort)
		dockerArgs := []string{"run", "-d", "--rm",
			"-e", fmt.Sprintf("TOKEN=%s", buildToken),
			"-p", portsMapping, "golangci/build-runner"}
		o.log.Infof("Docker setup args: %v", dockerArgs)
		cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf

		out, err := cmd.Output()
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("timeouted running container with args %v: err: %s, ctx err: %s, stderr: %s, stdout: %s",
					dockerArgs, err, ctx.Err(), stderrBuf.String(), string(out))
			}

			if i == attemptsN-1 {
				return nil, errors.Wrapf(err, "failed running container with args %s, stderr: %s, stdout: %s",
					dockerArgs, stderrBuf.String(), string(out))
			}

			o.log.Warnf("Failed running container with args %s, try again: err: %s, stderr: %s, stdout: %s",
				dockerArgs, err, stderrBuf.String(), string(out))
			time.Sleep(time.Second * 3)
			continue
		}

		outStr := strings.TrimSpace(string(out))
		return &SetupContainerResponse{
			ContainerID: ContainerID{
				ID:         outStr,
				Port:       port,
				BuildToken: buildToken,
			},
		}, nil
	}

	return nil, nil
}

func (o Orchestrator) handleShutdownRequest(w http.ResponseWriter, hr *http.Request) {
	o.wrapHTTPHandler(w, hr, func(hr *http.Request) (interface{}, error) {
		var req ShutdownContainerRequest
		if err := o.parseRequest(hr, &req); err != nil {
			return nil, err
		}

		if err := o.shutdownContainer(&req); err != nil {
			return nil, err
		}

		return &ShutdownContainerResponse{}, nil
	})
}

func (o Orchestrator) shutdownContainer(req *ShutdownContainerRequest) error {
	timeout := time.Millisecond * time.Duration(req.TimeoutMs)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	attemptsN := 3
	for i := 0; i < attemptsN; i++ {
		cmd := exec.CommandContext(ctx, "docker", "kill", req.ID)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("timeouted killing container: err: %s, ctx err: %s, out: %s",
					err, ctx.Err(), string(out))
			}

			if i == attemptsN-1 {
				return errors.Wrapf(err, "failed killing container, out: %s", string(out))
			}

			o.log.Warnf("Failed killing container, try again: err: %s, stderr: %s, stdout: %s",
				err, string(out))
			time.Sleep(time.Second * 3)
			continue
		}

		return nil
	}

	return nil
}

func (o Orchestrator) handleBuildCommandRequest(w http.ResponseWriter, hr *http.Request) {
	o.wrapHTTPHandler(w, hr, func(hr *http.Request) (interface{}, error) {
		var req BuildCommandRequest
		if err := o.parseRequest(hr, &req); err != nil {
			return nil, err
		}

		return o.runBuildCommand(&req)
	})
}

func (o Orchestrator) runBuildCommand(req *BuildCommandRequest) (*BuildCommandResponse, error) {
	cID := req.ContainerID
	if cID.Port < minDockerPort || cID.Port > maxDockerPort {
		return nil, fmt.Errorf("invalid port %d", cID.Port)
	}

	hrBody, err := json.Marshal(req.Request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to json marshal request")
	}

	buildRunnerAddr := fmt.Sprintf("http://127.0.0.1:%d/", cID.Port)
	hr, err := http.NewRequest(http.MethodPost, buildRunnerAddr, bytes.NewReader(hrBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make http request")
	}
	// no need for timeout here

	hr.Header.Add(build.TokenHeaderName, req.BuildToken)
	resp, err := http.DefaultClient.Do(hr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute http request to build runner %s", buildRunnerAddr)
	}

	if resp.Body == nil {
		return nil, errors.Wrapf(err, "no build runner %s response body", buildRunnerAddr)
	}
	defer resp.Body.Close()

	var res BuildCommandResponse
	if err = json.NewDecoder(resp.Body).Decode(&res.BuildResponse); err != nil {
		return nil, errors.Wrapf(err, "failed to json decode response from build runner %s", buildRunnerAddr)
	}

	return &res, nil
}
