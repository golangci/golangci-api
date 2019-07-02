package executors

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/buildagent/build"

	"github.com/levigross/grequests"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	uuid "github.com/satori/go.uuid"
)

type Fargate struct {
	envStore
	wd string

	containerAddr, taskID string
	token                 string

	log logutil.Log

	ecs *ecs.ECS
	ec2 *ec2.EC2
	cfg config.Config
}

var _ Executor = &Container{}

func NewFargate(log logutil.Log, cfg config.Config, awsSess *session.Session) *Fargate {
	f := &Fargate{
		log:   log,
		ecs:   ecs.New(awsSess),
		ec2:   ec2.New(awsSess),
		cfg:   cfg,
		token: uuid.NewV4().String(),
	}
	os.Setenv("FARGATE_CONTAINER_TOKEN"+f.token, f.token) // hide it in logs
	return f
}

func (f *Fargate) wrapExecutorError(err error) error {
	return errors.Wrap(ErrExecutorFail, err.Error())
}

func (f Fargate) runTask(ctx context.Context, req *Requirements) (*ecs.Task, error) {
	const defaultMemGB = 16
	taskDef := f.cfg.GetString("FARGATE_TASK_DEF")
	if req.MemoryGB > defaultMemGB {
		taskDef = f.cfg.GetString("FARGATE_MAX_MEM_TASK_DEF")
	}

	input := &ecs.RunTaskInput{
		Count: aws.Int64(1),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				AssignPublicIp: aws.String("ENABLED"),
				Subnets:        []*string{aws.String(f.cfg.GetString("FARGATE_SUBNET"))},
				SecurityGroups: []*string{aws.String(f.cfg.GetString("FARGATE_SEC_GROUP"))},
			},
		},
		LaunchType:     aws.String("FARGATE"),
		Cluster:        aws.String(f.cfg.GetString("FARGATE_CLUSTER")),
		TaskDefinition: aws.String(taskDef),
		Overrides: &ecs.TaskOverride{
			ContainerOverrides: []*ecs.ContainerOverride{
				{
					Name: aws.String(f.cfg.GetString("FARGATE_CONTAINER")),
					Environment: []*ecs.KeyValuePair{
						{
							Name:  aws.String("TOKEN"),
							Value: aws.String(f.token),
						},
					},
				},
			},
		},
	}

	res, err := f.ecs.RunTaskWithContext(ctx, input)
	if err != nil {
		return nil, errors.Wrap(f.wrapExecutorError(err), "failed to run aws fargate task")
	}

	if len(res.Failures) != 0 {
		return nil, errors.Wrapf(ErrExecutorFail, "failures during running aws fargate task: %#v", res.Failures)
	}

	if len(res.Tasks) != 1 {
		return nil, errors.Wrapf(ErrExecutorFail, "res.Tasks count != 1: %#v", res.Tasks)
	}

	return res.Tasks[0], nil
}

func (f Fargate) describeTask(ctx context.Context) (*ecs.Task, error) {
	input := &ecs.DescribeTasksInput{
		Cluster: aws.String(f.cfg.GetString("FARGATE_CLUSTER")),
		Tasks:   []*string{aws.String(f.taskID)},
	}

	res, err := f.ecs.DescribeTasksWithContext(ctx, input)
	if err != nil {
		return nil, errors.Wrap(f.wrapExecutorError(err), "failed to run aws fargate task")
	}

	if len(res.Failures) != 0 {
		return nil, errors.Wrapf(ErrExecutorFail, "failures during running aws fargate task: %#v", res.Failures)
	}

	if len(res.Tasks) != 1 {
		return nil, errors.Wrapf(ErrExecutorFail, "res.Tasks count != 1: %#v", res.Tasks)
	}

	return res.Tasks[0], nil
}

func (f *Fargate) waitForContainerStart(ctx context.Context) (*ecs.Task, error) {
	for {
		task, err := f.describeTask(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to describe task %s", f.taskID)
		}

		if task.LastStatus == nil {
			return nil, errors.Wrap(ErrExecutorFail, "failed to get task last status")
		}

		switch *task.LastStatus {
		case "PROVISIONING", "PENDING", "ACTIVATING":
			f.log.Infof("Waiting for container %s to start...", f.taskID)
			time.Sleep(f.cfg.GetDuration("FARGATE_POLL_DELAY", time.Second*10))
		case "RUNNING":
			f.log.Infof("Container started")
			return task, nil
		case "DEACTIVATING", "STOPPING", "DEPROVISIONING", "STOPPED":
			return nil, errors.Wrapf(ErrExecutorFail, "container is stopped or stopping: %s", *task.LastStatus)
		default:
			return nil, errors.Wrapf(ErrExecutorFail, "unknown task status %s", *task.LastStatus)
		}
	}
}

func (f *Fargate) describeNetworkInterface(ctx context.Context, networkInterfaceID string) (*ec2.NetworkInterface, error) {
	input := ec2.DescribeNetworkInterfacesInput{
		NetworkInterfaceIds: []*string{aws.String(networkInterfaceID)},
	}
	res, err := f.ec2.DescribeNetworkInterfacesWithContext(ctx, &input)
	if err != nil {
		return nil, errors.Wrapf(f.wrapExecutorError(err), "failed to describe network interface %s", networkInterfaceID)
	}

	if len(res.NetworkInterfaces) != 1 {
		return nil, errors.Wrapf(ErrExecutorFail, "len(res.NetworkInterfaces) != 1: %#v", res.NetworkInterfaces)
	}

	return res.NetworkInterfaces[0], nil
}

func (f *Fargate) getTaskPublicIP(ctx context.Context, task *ecs.Task) (string, error) {
	if len(task.Attachments) != 1 {
		return "", errors.Wrapf(ErrExecutorFail, "len(task.Attachments) != 1: %#v", task.Attachments)
	}

	attach := task.Attachments[0]
	var networkInterfaceID string
	for _, d := range attach.Details {
		if d.Name != nil && d.Value != nil && *d.Name == "networkInterfaceId" {
			networkInterfaceID = *d.Value
			break
		}
	}
	if networkInterfaceID == "" {
		return "", errors.Wrapf(ErrExecutorFail, "no networkInterfaceId in attachment details: %#v", attach)
	}

	ni, err := f.describeNetworkInterface(ctx, networkInterfaceID)
	if err != nil {
		return "", err
	}

	if ni.Association == nil {
		return "", errors.Wrapf(ErrExecutorFail, "no association in network interface %#v", ni)
	}

	if ni.Association.PublicIp == nil {
		return "", errors.Wrapf(ErrExecutorFail, "no public ip in network interface %#v", ni)
	}

	return *ni.Association.PublicIp, nil
}

func cleanEnvKey(s string) string {
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return s
}

func (f *Fargate) Setup(ctx context.Context, req *Requirements) error {
	task, err := f.runTask(ctx, req)
	if err != nil {
		return err
	}

	if task.TaskArn == nil {
		return errors.Wrapf(ErrExecutorFail, "no arn in task %#v", task)
	}
	f.taskID = *task.TaskArn

	os.Setenv("FARGATE_CONTAINER_"+cleanEnvKey(f.taskID), f.taskID) // hide it in logs

	task, err = f.waitForContainerStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to wait until container starts")
	}

	ip, err := f.getTaskPublicIP(ctx, task)
	if err != nil {
		return errors.Wrapf(err, "failed to get public ip for container of task %s", f.taskID)
	}
	os.Setenv("FARGATE_CONTAINER_IP"+cleanEnvKey(ip), ip) // hide it in logs
	f.containerAddr = fmt.Sprintf("http://%s:7000", ip)
	f.log.Infof("Fargate container started on addr %s", f.taskID)
	f.log.Infof("Started container with resource requirements: %#v", req)

	return nil
}

func (f Fargate) Run(ctx context.Context, name string, args ...string) (*RunResult, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("no deadline was set for context")
	}
	now := time.Now()
	if deadline.Before(now) {
		return nil, errors.Wrap(ErrExecutorFail, "deadline exceeded: it's before now")
	}

	req := build.Request{
		TimeoutMs: uint(deadline.Sub(now) / time.Millisecond),
		WorkDir:   f.wd,
		Env:       f.env,
		Kind:      build.RequestKindRun,
		Args:      append([]string{name}, args...),
	}

	return f.runBuildCommand(ctx, &req)
}

func (f Fargate) runBuildCommand(ctx context.Context, req *build.Request) (*RunResult, error) {
	resp, err := grequests.Post(fmt.Sprintf("%s/", f.containerAddr), &grequests.RequestOptions{
		Context: ctx,
		JSON:    req,
		Headers: map[string]string{
			build.TokenHeaderName: f.token,
		},
	})
	if err != nil {
		return nil, errors.Wrapf(f.wrapExecutorError(err), "failed to make request to build runner with req %#v", req)
	}

	var buildResp build.Response
	if err = resp.JSON(&buildResp); err != nil {
		return nil, errors.Wrap(f.wrapExecutorError(err), "failed to parse json of container response")
	}

	if buildResp.Error != "" {
		return nil, errors.Wrapf(ErrExecutorFail, "failed to run build command with req %#v: %s", req, buildResp.Error)
	}

	res := RunResult(buildResp.RequestResult)

	if buildResp.CommandError != "" {
		// no wrapping of ErrExecutorFail
		return &res, fmt.Errorf("build command for req %#v complete with error: %s",
			req, buildResp.CommandError)
	}

	return &res, nil
}

func (f Fargate) CopyFile(ctx context.Context, dst, src string) error {
	srcContent, err := ioutil.ReadFile(src)
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", src)
	}

	req := build.Request{
		WorkDir: f.wd,
		Env:     f.env,
		Kind:    build.RequestKindCopy,
		Args:    []string{dst, string(srcContent)},
	}

	_, err = f.runBuildCommand(ctx, &req)
	return err
}

func (f Fargate) Clean() {
	if f.taskID == "" {
		f.log.Infof("No need to stop fargate task: taskID is empty")
		return
	}

	_, err := f.ecs.StopTask(&ecs.StopTaskInput{
		Cluster: aws.String(f.cfg.GetString("FARGATE_CLUSTER")),
		Task:    aws.String(f.taskID),
	})
	if err != nil {
		f.log.Warnf("Failed to stop fargate task: %s", err)
		return
	}
	f.log.Infof("Stopped fargate task %s", f.taskID)
}

func (f Fargate) WithEnv(k, v string) Executor {
	eCopy := f
	eCopy.SetEnv(k, v)
	return &eCopy
}

func (f Fargate) WorkDir() string {
	return f.wd
}

func (f *Fargate) SetWorkDir(wd string) {
	f.wd = wd
}

func (f Fargate) WithWorkDir(wd string) Executor {
	eCopy := f
	eCopy.wd = wd
	return &eCopy
}
