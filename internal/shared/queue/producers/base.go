package producers

import (
	"github.com/golangci/golangci-api/internal/shared/queue"
	"github.com/pkg/errors"
)

type Base struct {
	q Queue
}

func (p *Base) Register(m *Multiplexer, queueID string) error {
	q, err := m.NewSubqueue(queueID)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s subqueue", queueID)
	}

	p.q = q
	return nil
}

func (p *Base) Put(message queue.Message) error {
	return p.q.Put(message)
}
