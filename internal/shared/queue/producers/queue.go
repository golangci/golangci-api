package producers

import "github.com/golangci/golangci-api/internal/shared/queue"

type Queue interface {
	Put(message queue.Message) error
}
