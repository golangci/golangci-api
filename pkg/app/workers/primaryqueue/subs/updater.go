package subs

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/paymentproviders"
	"github.com/golangci/golangci-api/pkg/app/paymentproviders/paymentprovider"
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

const updateQueueID = "subs/update"

type updateMessage struct {
	SubID uint
	Token string
	Seats int
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

func (cp UpdaterProducer) Put(subID uint, token string, seats int) error {
	return cp.Base.Put(updateMessage{
		SubID: subID,
		Token: token,
		Seats: seats,
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

	if err = cc.run(ctx, m, gormDB); err != nil {
		if errors.Cause(err) == consumers.ErrPermanent {
			n, err := models.NewOrgSubQuerySet(gormDB).IDEq(m.SubID).
				GetUpdater().
				SetCommitState(models.OrgSubCommitStateCreateDone).
				UpdateNum()
			if err != nil {
				cc.log.Warnf("failed to reset local sub %d state after remote update fail: %s", m.SubID, err.Error())
			} else if n == 0 {
				cc.log.Warnf("failed to reset local sub %d state after remote update fail", m.SubID)
			} else {
				// TODO(all): Should notify end-user about their failed subscription, probably email them...
			}
		}
		return errors.Wrapf(err, "create of sub %d failed", m.SubID)
	}

	return nil
}

func (cc UpdaterConsumer) run(ctx context.Context, m *updateMessage, db *gorm.DB) error {
	// TODO(all): Consider adding paymentprovider entry to sub table, for now defaulting to securionpay

	var sub models.OrgSub
	if err := models.NewOrgSubQuerySet(db).IDEq(m.SubID).One(&sub); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db sub with id %d", m.SubID)
		}
		return errors.Wrapf(err, "failed to fetch from db sub with id %d", m.SubID)
	}

	payments, err := cc.pp.Build("securionpay")
	if err != nil {
		return errors.Wrap(err, "failed to create payment gateway")
	}
	_, err = payments.UpdateSubscription(ctx, sub.PaymentGatewayCustomerID, sub.PaymentGatewaySubscriptionID, paymentprovider.SubscriptionUpdatePayload{
		CardToken:  m.Token,
		SeatsCount: m.Seats,
	})
	if err != nil {
		return errors.Wrap(consumers.ErrPermanent, "failed to update remote subscription")
	}

	query := models.NewOrgSubQuerySet(db).
		IDEq(sub.ID).CommitStateEq(models.OrgSubCommitStateUpdateSentToQueue).
		GetUpdater().
		SetCommitState(models.OrgSubCommitStateCreateDone)
	if m.Seats > 0 {
		query = query.SetSeatsCount(m.Seats)
	}
	if m.Token != "" {
		query = query.SetPaymentGatewayCardToken(m.Token)
	}
	n, err := query.UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update commit state to %s for sub with id %d",
			models.OrgSubCommitStateCreateDone, sub.ID)
	}
	if n == 0 {
		cc.log.Infof("Not updating sub %#v commit state to %s because it's already updated by queue consumer",
			sub, models.OrgSubCommitStateCreateDone)
	}

	return nil
}
