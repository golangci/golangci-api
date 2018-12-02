package subs

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/internal/api/paymentproviders"
	"github.com/golangci/golangci-api/internal/api/paymentproviders/paymentprovider"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

const updateQueueID = "subs/update"

type updateMessage struct {
	SubID      uint
	CardToken  string
	SeatsCount int
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

func (cp UpdaterProducer) Put(subID uint, seats int) error {
	return cp.Base.Put(updateMessage{
		SubID:      subID,
		SeatsCount: seats,
	})
}

type UpdaterConsumer struct {
	log logutil.Log
	db  *sql.DB
	cfg config.Config
	pp  paymentproviders.Factory
}

func NewUpdaterConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pp paymentproviders.Factory) *UpdaterConsumer {
	return &UpdaterConsumer{
		log: log,
		db:  db,
		cfg: cfg,
		pp:  pp,
	}
}

func (cc UpdaterConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(cc.consumeMessage, updateQueueID, m, df)
}

func (cc UpdaterConsumer) consumeMessage(ctx context.Context, m *updateMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, cc.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	return cc.run(ctx, m, gormDB)
}

func (cc UpdaterConsumer) run(ctx context.Context, m *updateMessage, db *gorm.DB) error {
	var sub models.OrgSub
	if err := models.NewOrgSubQuerySet(db).IDEq(m.SubID).One(&sub); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db sub with id %d", m.SubID)
		}
		return errors.Wrapf(err, "failed to fetch from db sub with id %d", m.SubID)
	}

	if sub.CommitState != models.OrgSubCommitStateUpdateInit && sub.CommitState != models.OrgSubCommitStateUpdateSentToQueue {
		return fmt.Errorf("got sub %d with invalid commit state %s", sub.ID, sub.CommitState)
	}

	payments, err := cc.pp.Build(sub.PaymentGatewayName)
	if err != nil {
		return errors.Wrap(err, "failed to create payment gateway")
	}

	_, err = payments.UpdateSubscription(ctx, sub.PaymentGatewayCustomerID, sub.PaymentGatewaySubscriptionID, paymentprovider.SubscriptionUpdatePayload{
		SeatsCount: m.SeatsCount,
		CardToken:  m.CardToken,
	})
	if err != nil {
		return errors.Wrap(err, "failed to update remote subscription")
	}

	query := models.NewOrgSubQuerySet(db).
		IDEq(sub.ID).
		CommitStateIn(models.OrgSubCommitStateUpdateInit, models.OrgSubCommitStateUpdateSentToQueue).
		VersionEq(sub.Version).
		GetUpdater().
		SetCommitState(models.OrgSubCommitStateUpdateDone).
		SetSeatsCount(m.SeatsCount).
		SetVersion(sub.Version + 1)

	if m.CardToken != "" {
		query = query.SetPaymentGatewayCardToken(m.CardToken)
	}
	if err = query.UpdateRequired(); err != nil {
		return errors.Wrapf(err, "failed to update commit state to %s for sub with id %d",
			models.OrgSubCommitStateCreateDone, sub.ID)
	}

	cc.log.Infof("Successfully updated subscription %d from %d to %d seats", sub.ID, sub.SeatsCount, m.SeatsCount)
	return nil
}
