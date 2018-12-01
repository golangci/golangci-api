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
		if errors.Cause(err) == consumers.ErrPermanent {
			if err = models.NewOrgSubQuerySet(gormDB).IDEq(m.SubID).Delete(); err != nil {
				cc.log.Warnf("failed to delete local sub %d after remote create fail: %s", m.SubID, err.Error())
			}
			// TODO(all): Should notify end-user about their failed subscription, probably email them...
		}
		return errors.Wrapf(err, "create of sub %d failed", m.SubID)
	}

	return nil
}

func (cc CreatorConsumer) run(ctx context.Context, m *createMessage, db *gorm.DB) error {
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

	if sub.PaymentGatewayCustomerID == "" {
		if err = cc.createCustomer(ctx, db, &sub, payments); err != nil {
			return errors.Wrap(err, "failed to create customer")
		}
	}

	if sub.PaymentGatewaySubscriptionID == "" {
		if err = cc.createSubscription(ctx, db, &sub, payments); err != nil {
			return errors.Wrap(err, "failed to create subscription")
		}
	}

	n, err := models.NewOrgSubQuerySet(db).
		IDEq(sub.ID).CommitStateEq(models.OrgSubCommitStateCreateSentToQueue).
		GetUpdater().
		SetCommitState(models.OrgSubCommitStateCreateDone).
		UpdateNum()
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

func (cc CreatorConsumer) createCustomer(ctx context.Context, db *gorm.DB, sub *models.OrgSub, payments paymentprovider.Provider) error {
	var user models.User
	if err := models.NewUserQuerySet(db).IDEq(sub.BillingUserID).One(&user); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db user via billing user id %d", sub.BillingUserID)
		}
		return errors.Wrapf(err, "failed to fetch from db user via billing user id %d", sub.BillingUserID)
	}
	if user.Email == "" {
		return errors.Errorf("expected email for user with id %d not to be empty", user.ID)
	}
	cust, err := payments.CreateCustomer(ctx, user.Email, sub.PaymentGatewayCardToken)
	if err != nil {
		if err == paymentprovider.ErrInvalidCardToken {
			return errors.Wrapf(consumers.ErrPermanent, "call to create customer for sub:%d user:%d failed", sub.ID, user.ID)
		}
		return errors.Wrapf(err, "call to create customer for sub:%d user:%d failed", sub.ID, user.ID)
	}
	sub.PaymentGatewayCustomerID = cust.ID
	n, err := models.NewOrgSubQuerySet(db).IDEq(sub.ID).GetUpdater().SetPaymentGatewayCustomerID(cust.ID).UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update sub with id %d", sub.ID)
	} else if n != 1 {
		return errors.New("no rows were updated, this really shouldn't happen")
	}
	return nil
}

func (cc CreatorConsumer) createSubscription(ctx context.Context, db *gorm.DB, sub *models.OrgSub, payments paymentprovider.Provider) error {
	psub, err := payments.CreateSubscription(ctx, sub.PaymentGatewayCustomerID, sub.SeatsCount)
	if err != nil {
		return errors.Wrapf(err, "call to payment gateway to create subscription for sub:%d failed", sub.ID)
	}
	sub.PaymentGatewaySubscriptionID = psub.ID
	n, err := models.NewOrgSubQuerySet(db).IDEq(sub.ID).GetUpdater().SetPaymentGatewaySubscriptionID(psub.ID).UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update sub with id %d", sub.ID)
	} else if n != 1 {
		return errors.New("no rows were updated, this really shouldn't happen")
	}
	return nil
}
