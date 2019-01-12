package app

import (
	"testing"
	"time"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/pullanalyzesqueue"

	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"
	"github.com/golangci/golangci-api/pkg/worker/test"
	"github.com/stretchr/testify/assert"
)

type testProcessor struct {
	notifyCh chan bool
}

func (tp testProcessor) Process(*processors.PullContext) error {
	tp.notifyCh <- true
	return nil
}

type testProcessorFatory struct {
	t               *testing.T
	expAnalysisGUID string
	notifyCh        chan bool
}

func (tpf testProcessorFatory) BuildProcessor(ctx *processors.PullContext) (processors.PullProcessor, func(), error) {
	assert.Equal(tpf.t, tpf.expAnalysisGUID, ctx.AnalysisGUID)
	return testProcessor{
		notifyCh: tpf.notifyCh,
	}, nil, nil
}

func TestSendReceiveProcessing(t *testing.T) {
	notifyCh := make(chan bool)
	testGUID := "test"
	pf := testProcessorFatory{
		t:               t,
		expAnalysisGUID: testGUID,
		notifyCh:        notifyCh,
	}

	test.Init()
	a := NewApp(SetPullProcessorFactory(pf))
	go a.Run()

	testDeps := a.BuildTestDeps()
	msg := pullanalyzesqueue.RunMessage{
		AnalysisGUID: testGUID,
	}
	assert.NoError(t, testDeps.PullAnalyzesRunner.Put(&msg))

	select {
	case <-notifyCh:
		return
	case <-time.After(time.Second * 3):
		t.Fatalf("Timeouted waiting of processing")
	}
}
