package paymentevents

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/implementations/paddle"
	"github.com/golangci/golangci-api/internal/api/paymentproviders/paymentprovider"

	"github.com/golangci/golangci-api/internal/api/paymentproviders"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	redsync "gopkg.in/redsync.v1"
)

const createQueueID = "payment/events/create"

type createMessage struct {
	Provider string
	Payload  string
	UUID     string
}

func (m createMessage) LockID() string {
	// TODO(d.isaev): maybe lock for user?
	return fmt.Sprintf("%s/%s/%s", createQueueID, m.Provider, m.UUID)
}

type CreatorProducer struct {
	producers.Base
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, createQueueID)
}

func (cp CreatorProducer) Put(provider, payload string) error {
	return cp.Base.Put(createMessage{
		Provider: provider,
		Payload:  payload,
		UUID:     uuid.NewV4().String(),
	})
}

type CreatorConsumer struct {
	log logutil.Log
	db  *sql.DB
	cfg config.Config
	pp  paymentproviders.Factory
}

func NewCreatorConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pp paymentproviders.Factory) *CreatorConsumer {
	return &CreatorConsumer{
		log: log,
		db:  db,
		cfg: cfg,
		pp:  pp,
	}
}

func (cc CreatorConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(cc.consumeMessage, createQueueID, m, df)
}

func (cc CreatorConsumer) consumeMessage(ctx context.Context, m *createMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, cc.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	if err = cc.run(ctx, m, gormDB); err != nil {
		return errors.Wrapf(err, "create of event for %s failed", m.Provider)
	}

	return nil
}

func (cc CreatorConsumer) run(_ context.Context, m *createMessage, db *gorm.DB) (retErr error) {
	tx, finish, err := gormdb.StartTx(db)
	if err != nil {
		return errors.Wrap(err, "failed to start tx")
	}
	defer finish(&retErr)

	var ep paymentprovider.EventProcessor
	switch m.Provider {
	case paddle.ProviderName:
		ep = &paddle.EventProcessor{
			Tx:  tx,
			Log: cc.log,
		}
	}

	if err := ep.Process(m.Payload, m.UUID); err != nil {
		return errors.Wrapf(err, "failed to process by %s event processor", m.Provider)
	}

	return nil
}
