package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

type Multiplexer struct {
	consumers map[string]Consumer
}

func NewMultiplexer() *Multiplexer {
	return &Multiplexer{
		consumers: map[string]Consumer{},
	}
}

type subconsumerMessage struct {
	SubqueueID string
	Message    json.RawMessage
}

func (m Multiplexer) consumerNames() []string {
	var ret []string
	for name := range m.consumers {
		ret = append(ret, name)
	}

	return ret
}

func (m *Multiplexer) ConsumeMessage(ctx context.Context, message []byte) error {
	var sm subconsumerMessage
	if err := json.Unmarshal(message, &sm); err != nil {
		return errors.Wrap(err, "json unmarshal failed")
	}

	consumer := m.consumers[sm.SubqueueID]
	if consumer == nil {
		return fmt.Errorf("no consumer with id %s, registered consumers: %v", sm.SubqueueID, m.consumerNames())
	}

	return consumer.ConsumeMessage(ctx, []byte(sm.Message))
}

func (m *Multiplexer) RegisterConsumer(id string, consumer Consumer) error {
	if m.consumers[id] != nil {
		return fmt.Errorf("consumer %s is already registered", id)
	}
	m.consumers[id] = consumer

	return nil
}
