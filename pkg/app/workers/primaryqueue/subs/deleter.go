package subs

import (
	"fmt"

	"github.com/golangci/golangci-api/pkg/queue/producers"
)

const deleteQueueID = "subs/delete"

type deleteMessage struct {
	SubID uint
}

func (m deleteMessage) LockID() string {
	return fmt.Sprintf("%s/%d", deleteQueueID, m.SubID)
}

type DeleterProducer struct {
	producers.Base
}

func (cp *DeleterProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, deleteQueueID)
}

func (cp DeleterProducer) Put(subID uint) error {
	return cp.Base.Put(deleteMessage{
		SubID: subID,
	})
}
