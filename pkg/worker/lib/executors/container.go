package executors

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/buildagent/build"
	"github.com/golangci/golangci-api/pkg/buildagent/containers"

	"github.com/levigross/grequests"
	"github.com/pkg/errors"
)

type Container struct {
	envStore
	wd string

	orchestratorAddr string
	token            string

	containerID containers.ContainerID
	log         logutil.Log
}

var _ Executor = &Container{}

func NewContainer(log logutil.Log) (*Container, error) {
	orchestratorAddr := os.Getenv("ORCHESTRATOR_ADDR")
	if orchestratorAddr == "" {
		return nil, errors.New("no ORCHESTRATOR_ADDR env var")
	}
	if strings.HasSuffix(orchestratorAddr, "/") {
		return nil, errors.New("ORCHESTRATOR_ADDR shouldn't end with /")
	}

	token := os.Getenv("ORCHESTRATOR_TOKEN")
	if token == "" {
		return nil, errors.New("no ORCHESTRATOR_TOKEN env var")
	}

	return &Container{
		envStore:         envStore{},
		orchestratorAddr: orchestratorAddr,
		token:            token,
		log:              log,
	}, nil
}

func (c *Container) Setup(ctx context.Context) error {
	resp, err := grequests.Post(fmt.Sprintf("%s/setup", c.orchestratorAddr), &grequests.RequestOptions{
		Context: ctx,
		JSON: containers.SetupContainerRequest{
			TimeoutMs: 30 * 1000, // 30s
		},
		Headers: map[string]string{
			containers.TokenHeaderName: c.token,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to make request to orchestrator")
	}

	var setupResp containers.SetupContainerResponse
	if err = resp.JSON(&setupResp); err != nil {
		return errors.Wrap(err, "failed to parse json of setup response")
	}

	if setupResp.Error != "" {
		return fmt.Errorf("failed to setup container: %s", setupResp.Error)
	}

	c.log.Infof("Setup of container: id is %#v", setupResp.ContainerID)
	c.containerID = setupResp.ContainerID

	// to hide it in logs
	os.Setenv("container_id_"+c.containerID.ID, c.containerID.ID)
	os.Setenv("container_token_"+c.containerID.BuildToken, c.containerID.BuildToken)

	return nil
}

func (c Container) Run(ctx context.Context, name string, args ...string) (string, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return "", errors.New("no deadline was set for context")
	}
	now := time.Now()
	if deadline.Before(now) {
		return "", errors.New("deadline exceeded: it's before now")
	}

	req := containers.BuildCommandRequest{
		ContainerID: c.containerID,
		Request: build.Request{
			TimeoutMs: uint(deadline.Sub(now) / time.Millisecond),
			WorkDir:   c.wd,
			Env:       c.env,
			Kind:      build.RequestKindRun,
			Args:      append([]string{name}, args...),
		},
	}

	return c.runBuildCommand(ctx, &req)
}

func (c Container) runBuildCommand(ctx context.Context, req *containers.BuildCommandRequest) (string, error) {
	resp, err := grequests.Post(fmt.Sprintf("%s/buildcommand", c.orchestratorAddr), &grequests.RequestOptions{
		Context: ctx,
		JSON:    req,
		Headers: map[string]string{
			containers.TokenHeaderName: c.token,
		},
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to make request to orchestrator with req %#v", req)
	}

	var containerResp containers.BuildCommandResponse
	if err = resp.JSON(&containerResp); err != nil {
		return "", errors.Wrap(err, "failed to parse json of container response")
	}

	if containerResp.Error != "" {
		return "", fmt.Errorf("failed to run container build command with req %#v: %s",
			req, containerResp.Error)
	}

	buildResp := containerResp.BuildResponse
	if buildResp.Error != "" {
		return "", fmt.Errorf("failed to run build command with req %#v: %s", req, buildResp.Error)
	}

	if buildResp.CommandError != "" {
		return buildResp.StdOut, fmt.Errorf("build command for req %#v complete with error: %s",
			req, buildResp.CommandError)
	}

	return buildResp.StdOut, nil
}

func (c Container) CopyFile(ctx context.Context, dst, src string) error {
	srcContent, err := ioutil.ReadFile(src)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", src)
	}

	req := containers.BuildCommandRequest{
		ContainerID: c.containerID,
		Request: build.Request{
			WorkDir: c.wd,
			Env:     c.env,
			Kind:    build.RequestKindCopy,
			Args:    []string{dst, string(srcContent)},
		},
	}

	_, err = c.runBuildCommand(ctx, &req)
	return err
}

func (c Container) Clean() {
	err := func() error {
		ctx, finish := context.WithTimeout(context.TODO(), time.Second*30)
		defer finish()

		resp, err := grequests.Post(fmt.Sprintf("%s/shutdown", c.orchestratorAddr), &grequests.RequestOptions{
			Context: ctx,
			JSON: containers.ShutdownContainerRequest{
				TimeoutMs:   30 * 1000, // 30s
				ContainerID: c.containerID,
			},
			Headers: map[string]string{
				containers.TokenHeaderName: c.token,
			},
		})
		if err != nil {
			return errors.Wrap(err, "failed to make request to orchestrator")
		}

		var shutdownResp containers.ShutdownContainerResponse
		if err = resp.JSON(&shutdownResp); err != nil {
			return errors.Wrap(err, "failed to parse json of shutdown response")
		}

		if shutdownResp.Error != "" {
			return fmt.Errorf("failed to shutdown container: %s", shutdownResp.Error)
		}

		c.log.Infof("Shutdowned container with id %#v", c.containerID)
		return nil
	}()
	if err != nil {
		c.log.Warnf("Failed to shutdown container: %s", err)
	}
}

func (c Container) WithEnv(k, v string) Executor {
	eCopy := c
	eCopy.SetEnv(k, v)
	return &eCopy
}

func (c Container) WorkDir() string {
	return c.wd
}

func (c *Container) SetWorkDir(wd string) {
	c.wd = wd
}

func (c Container) WithWorkDir(wd string) Executor {
	eCopy := c
	eCopy.wd = wd
	return &eCopy
}
