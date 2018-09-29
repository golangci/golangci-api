package producers

import "github.com/golangci/golangci-api/pkg/queue"

type Queue interface {
	Put(message queue.Message) error
}
