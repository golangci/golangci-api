package pullanalyzesqueue

import (
	"github.com/golangci/golangci-api/internal/shared/queue"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
)

type Producer struct {
	producers.Base
}

func (p *Producer) Register(m *producers.Multiplexer) error {
	return p.Base.Register(m, runQueueID)
}

func (p Producer) Put(m queue.Message) error {
	return p.Base.Put(m)
}
