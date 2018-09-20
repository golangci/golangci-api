package producers

import (
	"fmt"
)

type Multiplexer struct {
	q         Queue
	subqueues map[string]bool
}

func NewMultiplexer(q Queue) *Multiplexer {
	return &Multiplexer{
		q:         q,
		subqueues: map[string]bool{},
	}
}

type subqueue struct {
	id     string
	parent *Multiplexer
}

type subqueueMessage struct {
	SubqueueID string
	Message    Message
}

func (sm subqueueMessage) DeduplicationID() string {
	return sm.Message.DeduplicationID()
}

func (sq subqueue) Put(message Message) error {
	return sq.parent.q.Put(subqueueMessage{
		SubqueueID: sq.id,
		Message:    message,
	})
}

func (m *Multiplexer) NewSubqueue(id string) (Queue, error) {
	if m.subqueues[id] {
		return nil, fmt.Errorf("subqueue %s is already registered", id)
	}
	m.subqueues[id] = true

	return &subqueue{
		id:     id,
		parent: m,
	}, nil
}
