package primaryqueue

import (
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

func RegisterConsumer(consumeFunc interface{}, queueID string, m *consumers.Multiplexer, df *redsync.Redsync) error {
	consumer, err := consumers.NewReflectConsumer(consumeFunc, ConsumerTimeout, df)
	if err != nil {
		return errors.Wrap(err, "can't make reflect consumer")
	}

	return m.RegisterConsumer(queueID, consumer)
}
