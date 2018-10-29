package subs

import (
	"fmt"

	"github.com/golangci/golangci-api/pkg/queue/producers"
)

const updateQueueID = "subs/update"

type updateMessage struct {
	SubID uint
}

func (m updateMessage) LockID() string {
	return fmt.Sprintf("%s/%d", updateQueueID, m.SubID)
}

type UpdaterProducer struct {
	producers.Base
}

func (cp *UpdaterProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, updateQueueID)
}

func (cp UpdaterProducer) Put(subID uint) error {
	return cp.Base.Put(updateMessage{
		SubID: subID,
	})
}
