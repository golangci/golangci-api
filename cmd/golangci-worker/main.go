package main

import (
	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzequeue"
	"github.com/golangci/golangci-api/pkg/worker/lib/queue"
	"github.com/sirupsen/logrus"
)

func main() {
	queue.Init()
	analyzequeue.RegisterTasks()
	if err := analyzequeue.RunWorker(); err != nil {
		logrus.Fatalf("Can't run analyze worker: %s", err)
	}
}
