package subs

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

const createQueueID = "subs/create"

type createMessage struct {
	SubID uint
}

func (m createMessage) LockID() string {
	return fmt.Sprintf("%s/%d", createQueueID, m.SubID)
}

type CreatorProducer struct {
	producers.Base
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, createQueueID)
}

func (cp CreatorProducer) Put(subID uint) error {
	return cp.Base.Put(createMessage{
		SubID: subID,
	})
}

type CreatorConsumer struct {
	log logutil.Log
	db  *sql.DB
	cfg config.Config
}

func NewCreatorConsumer(log logutil.Log, db *sql.DB, cfg config.Config) *CreatorConsumer {
	return &CreatorConsumer{
		log: log,
		db:  db,
		cfg: cfg,
	}
}

func (cc CreatorConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(cc.consumeMessage, createQueueID, m, df)
}

func (cc CreatorConsumer) consumeMessage(_ context.Context, m *createMessage) error {
	if m == nil {
		return errors.New("just a temp")
	}
	cc.log.Warnf("got a creator message %#v", *m)
	// gormDB, err := gormdb.FromSQL(ctx, cc.db)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to get gorm db")
	// }

	// if err = cc.run(ctx, m, gormDB); err != nil {
	// 	return errors.Wrapf(err, "create of repo %d failed", m.RepoID)
	// }

	return nil
}
