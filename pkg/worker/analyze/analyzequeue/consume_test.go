package analyzequeue

import (
	"context"
	"testing"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/consumers"
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue/task"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/lib/queue"
	"github.com/golangci/golangci-api/pkg/worker/test"
	"github.com/stretchr/testify/assert"
)

type processorMocker struct {
	prevProcessorFactory processors.Factory
}

func (pm processorMocker) restore() {
	consumers.ProcessorFactory = pm.prevProcessorFactory
}

func mockProcessor(newProcessorFactory processors.Factory) *processorMocker {
	ret := &processorMocker{
		prevProcessorFactory: newProcessorFactory,
	}
	consumers.ProcessorFactory = newProcessorFactory
	return ret
}

type testProcessor struct {
	notifyCh chan bool
}

func (tp testProcessor) Process(ctx context.Context) error {
	tp.notifyCh <- true
	return nil
}

type testProcessorFatory struct {
	t        *testing.T
	expTask  *task.PRAnalysis
	notifyCh chan bool
}

func (tpf testProcessorFatory) BuildProcessor(ctx context.Context, t *task.PRAnalysis) (processors.Processor, error) {
	assert.Equal(tpf.t, tpf.expTask, t)
	return testProcessor{
		notifyCh: tpf.notifyCh,
	}, nil
}
func TestSendReceiveProcessing(t *testing.T) {
	task := &task.PRAnalysis{
		Context:      github.FakeContext,
		APIRequestID: "req_id",
	}

	notifyCh := make(chan bool)
	defer mockProcessor(testProcessorFatory{
		t:        t,
		expTask:  task,
		notifyCh: notifyCh,
	}).restore()

	test.Init()
	queue.Init()
	RegisterTasks()
	go func() {
		err := RunWorker()
		assert.NoError(t, err)
	}()

	assert.NoError(t, SchedulePRAnalysis(task))

	select {
	case <-notifyCh:
		return
	case <-time.After(time.Second * 1):
		t.Fatalf("Timeouted waiting of processing")
	}
}
