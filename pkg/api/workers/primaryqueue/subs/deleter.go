package subs

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/api/paymentproviders"
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

type DeleterConsumer struct {
	log logutil.Log
	db  *sql.DB
	cfg config.Config
	pp  paymentproviders.Factory
}

func NewDeleterConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pp paymentproviders.Factory) *DeleterConsumer {
	return &DeleterConsumer{
		log: log,
		db:  db,
		cfg: cfg,
		pp:  pp,
	}
}

func (cc DeleterConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(cc.consumeMessage, deleteQueueID, m, df)
}

//nolint:dupl
func (cc DeleterConsumer) consumeMessage(ctx context.Context, m *deleteMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, cc.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	if err = cc.run(ctx, m, gormDB); err != nil {
		if errors.Cause(err) == consumers.ErrPermanent {
			var n int64
			n, err = models.NewOrgSubQuerySet(gormDB).IDEq(m.SubID).
				GetUpdater().
				SetCommitState(models.OrgSubCommitStateCreateDone).
				UpdateNum()
			if err != nil {
				cc.log.Warnf("failed to reset local sub %d state after remote update fail: %s", m.SubID, err.Error())
			} else if n == 0 {
				cc.log.Warnf("failed to reset local sub %d state after remote update fail", m.SubID)
			}
			// TODO(all): Should notify end-user about their failed subscription, probably email them...
		}
		return errors.Wrapf(err, "create of sub %d failed", m.SubID)
	}

	return nil
}

func (cc DeleterConsumer) run(ctx context.Context, m *deleteMessage, db *gorm.DB) error {
	// TODO(all): Consider adding paymentprovider entry to sub table, for now defaulting to securionpay

	var sub models.OrgSub
	if err := models.NewOrgSubQuerySet(db).IDEq(m.SubID).One(&sub); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db sub with id %d", m.SubID)
		}
		return errors.Wrapf(err, "failed to fetch from db sub with id %d", m.SubID)
	}

	if sub.DeletedAt != nil {
		cc.log.Warnf("Sub %d is already deleted", m.SubID)
	}

	payments, err := cc.pp.Build("securionpay")
	if err != nil {
		return errors.Wrap(err, "failed to create payment gateway")
	}
	err = payments.DeleteSubscription(ctx, sub.PaymentGatewayCustomerID, sub.PaymentGatewaySubscriptionID)
	if err != nil {
		return errors.Wrap(consumers.ErrPermanent, "failed to delete remote subscription")
	}

	now := time.Now()
	n, err := models.NewOrgSubQuerySet(db).IDEq(sub.ID).
		CommitStateIn(models.OrgSubCommitStateDeleteInit, models.OrgSubCommitStateDeleteSentToQueue).
		GetUpdater().
		SetCommitState(models.OrgSubCommitStateDeleteDone).
		SetDeletedAt(&now).
		UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update commit state to %s for sub with id %d",
			models.OrgSubCommitStateDeleteDone, sub.ID)
	}
	if n == 0 {
		cc.log.Infof("Not updating sub %#v commit state to %s because it's already updated by queue consumer",
			sub, models.OrgSubCommitStateDeleteDone)
	}

	return nil
}
