package paymentevents

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/paymentproviders"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

const createQueueID = "payment/events/create"

type createMessage struct {
	Provider string
	EventID  string
}

func (m createMessage) LockID() string {
	return fmt.Sprintf("%s/%s/%s", createQueueID, m.Provider, m.EventID)
}

type CreatorProducer struct {
	producers.Base
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, createQueueID)
}

func (cp CreatorProducer) Put(provider, event string) error {
	return cp.Base.Put(createMessage{
		Provider: provider,
		EventID:  event,
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
		return errors.Wrapf(err, "create of event %#v for %s failed", m.EventID, m.Provider)
	}

	return nil
}

func (cc CreatorConsumer) run(ctx context.Context, m *createMessage, db *gorm.DB) error {
	payments, err := cc.pp.Build(m.Provider)
	if err != nil {
		return errors.Wrap(err, "failed to create payment gateway provider")
	}

	event, err := payments.GetEvent(ctx, m.EventID)
	if err != nil {
		return errors.Wrap(err, "failed to fetch event from provider")
	}

	dbEvent := &models.PaymentGatewayEvent{
		Provider:   m.Provider,
		ProviderID: m.EventID,

		Type: event.Type,
		Data: event.Data,
	}
	if err := dbEvent.Create(db); err != nil {
		return errors.Wrap(err, "failed to save event to db")
	}

	return nil
}
