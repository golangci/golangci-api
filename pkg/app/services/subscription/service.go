package subscription

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/paymentproviders/implementations"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
	"github.com/golangci/golangci-api/pkg/app/returntypes"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue/paymentevents"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue/subs"
	"github.com/golangci/golangci-api/pkg/cache"
	"github.com/golangci/golangci-api/pkg/endpoint/apierrors"
	"github.com/golangci/golangci-api/pkg/endpoint/request"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type SubPayload struct {
	PaymentGatewayCardToken string `json:"payment_gateway_card_token"`
	SeatsCount              int    `json:"seatsCount"`
	IdempotencyKey          string `json:"idempotency_key"`
}

func (r SubPayload) FillLogContext(lctx logutil.Context) {
	if r.SeatsCount > 0 {
		lctx["seats_count"] = r.SeatsCount
	}
}

type WrappedSubInfo struct {
	Subscription         *returntypes.SubInfo `json:"subscription,omitempty"`
	TrialAllowanceInDays *int                 `json:"trialAllowanceInDays,omitempty"`
}

func newTrialSubInfo(days int) *WrappedSubInfo {
	return &WrappedSubInfo{TrialAllowanceInDays: &days}
}

type EventRequestContext struct {
	Provider string `request:"provider,urlPart,"`
}

func (r EventRequestContext) FillLogContext(lctx logutil.Context) {
	lctx["provider"] = r.Provider
}

type EventRequestPayload struct {
	ID string `json:"id"`
}

func (r EventRequestPayload) FillLogContext(lctx logutil.Context) {
	lctx["event"] = r.ID
}

// TODO: Should probably move payment gateway webhook to a separate service
// But it's a single endpoint so idk...

type Service interface {
	//url:/v1/orgs/{org_id}/subs
	List(rc *request.AuthorizedContext, context *request.OrgID) (*WrappedSubInfo, error)

	//url:/v1/orgs/{org_id}/subs/{sub_id}
	Get(rc *request.AuthorizedContext, context *request.OrgSubID) (*returntypes.SubInfo, error)

	//url:/v1/orgs/{org_id}/subs method:POST
	Create(rc *request.AuthorizedContext, context *request.OrgID, payload *SubPayload) (*returntypes.SubInfo, error)

	//url:/v1/orgs/{org_id}/subs/{sub_id} method:PUT
	Update(rc *request.AuthorizedContext, context *request.OrgSubID, payload *SubPayload) error

	//url:/v1/orgs/{org_id}/subs/{sub_id} method:DELETE
	Delete(rc *request.AuthorizedContext, context *request.OrgSubID) error

	//url:/v1/payments/{provider}/events method:POST
	EventCreate(rc *request.AnonymousContext, context *EventRequestContext, payload *EventRequestPayload) error
}

func Configure(providerFactory providers.Factory, cache cache.Cache, cfg config.Config,
	create *subs.CreatorProducer, delete *subs.DeleterProducer, update *subs.UpdaterProducer, pec *paymentevents.CreatorProducer) Service {
	return &basicService{providerFactory, cache, cfg, create, delete, update, pec}
}

type basicService struct {
	ProviderFactory providers.Factory
	Cache           cache.Cache
	Cfg             config.Config

	CreateQueue *subs.CreatorProducer
	DeleteQueue *subs.DeleterProducer
	UpdateQueue *subs.UpdaterProducer

	EventCreateQueue *paymentevents.CreatorProducer
}

// Find existing subscription for the organization and return it in "subscription" field (if subscription exists).
// If no subscription return trial duration: defend from always recreating subscription with the trial: select created_at
// of first created subscription and get trial duration as (config.getTrialDuration() - (now() -
// first_subscription.created_at).
func (s *basicService) List(rc *request.AuthorizedContext, context *request.OrgID) (*WrappedSubInfo, error) {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID).One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to get org from db")
	}
	if err := s.isAdminCached(rc, &org); err != nil {
		return nil, errors.Wrap(err, "failed to check for admin")
	}
	var sub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).OrgIDEq(context.OrgID).One(&sub)

	trialPeriod := s.Cfg.GetDuration("SUB_TRIAL_PERIOD", time.Hour*24*7)

	if err == gorm.ErrRecordNotFound {
		// No sub, try to get oldest probably deleted sub and use that in math for trial.
		err = models.NewOrgSubQuerySet(rc.DB.Unscoped()).OrgIDEq(context.OrgID).One(&sub)
		if err == gorm.ErrRecordNotFound {
			return newTrialSubInfo(int(trialPeriod.Hours() / 24)), nil
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to fetch oldest unscoped org sub")
		}

		return newTrialSubInfo(int((trialPeriod - time.Since(sub.CreatedAt)).Hours() / 24)), nil
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to fetch scoped org sub")
	}

	return &WrappedSubInfo{Subscription: returntypes.SubFromModel(sub)}, nil
}

func (s *basicService) Get(rc *request.AuthorizedContext, context *request.OrgSubID) (*returntypes.SubInfo, error) {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID.OrgID).One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to get org from db")
	}
	if err := s.isAdminCached(rc, &org); err != nil {
		return nil, errors.Wrap(err, "failed to check for admin")
	}

	var sub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).IDEq(context.SubID.SubID).One(&sub)
	if err == gorm.ErrRecordNotFound {
		return nil, apierrors.ErrNotFound
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to fetch scoped org sub")
	}

	return returntypes.SubFromModel(sub), nil
}

func (s basicService) finishQueueSending(rc *request.AuthorizedContext, sub *models.OrgSub,
	expState, setState models.OrgSubCommitState) (*returntypes.SubInfo, error) {

	n, err := models.NewOrgSubQuerySet(rc.DB).
		IDEq(sub.ID).CommitStateEq(expState).
		GetUpdater().
		SetCommitState(setState).
		UpdateNum()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update commit state to %s for sub with id %d",
			setState, sub.ID)
	}
	if n == 0 {
		rc.Log.Infof("Not updating sub %#v commit state to %s because it's already updated by queue consumer",
			sub, setState)
	}
	sub.CommitState = setState

	return returntypes.SubFromModel(*sub), nil
}

func (s basicService) sendToCreateQueue(rc *request.AuthorizedContext, sub *models.OrgSub) (*returntypes.SubInfo, error) {
	if err := s.CreateQueue.Put(sub.ID); err != nil {
		return nil, errors.Wrap(err, "failed to put to create subs queue")
	}
	return s.finishQueueSending(rc, sub, models.OrgSubCommitStateCreateInit, models.OrgSubCommitStateCreateSentToQueue)
}

//nolint:gocyclo
func (s *basicService) Create(rc *request.AuthorizedContext, context *request.OrgID, payload *SubPayload) (*returntypes.SubInfo, error) {
	if payload.PaymentGatewayCardToken == "" || payload.IdempotencyKey == "" {
		return nil, errors.New("idempotency key and card token are required for new subscriptions")
	}

	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID).One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to get org from db")
	}
	if err := s.isAdminCached(rc, &org); err != nil {
		return nil, errors.Wrap(err, "failed to check for admin")
	}

	var retSub *returntypes.SubInfo

	var orgSub models.OrgSub
	if err := models.NewOrgSubQuerySet(rc.DB).IdempotencyKeyEq(payload.IdempotencyKey).One(&orgSub); err == gorm.ErrRecordNotFound {
		// Doesn't exist, carry on...
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to check for idempotency key")
	} else {
		switch orgSub.CommitState {
		case models.OrgSubCommitStateCreateInit:
			retSub, err = s.sendToCreateQueue(rc, &orgSub)
			if err != nil {
				return nil, err
			}
			return retSub, nil
		default:
			return returntypes.SubFromModel(orgSub), nil
		}
	}

	sub := models.OrgSub{
		OrgID:                   context.OrgID,
		BillingUserID:           rc.User.ID,
		PaymentGatewayCardToken: payload.PaymentGatewayCardToken,
		SeatsCount:              payload.SeatsCount,
		CommitState:             models.OrgSubCommitStateCreateInit,
		IdempotencyKey:          payload.IdempotencyKey,
	}

	var err error
	if err = sub.Create(rc.DB); err != nil {
		return nil, errors.Wrap(err, "can't create sub")
	}

	retSub, err = s.sendToCreateQueue(rc, &sub)
	if err != nil {
		return nil, err
	}

	rc.Log.Infof("Created sub %#v", retSub)
	return retSub, nil
}

func (s basicService) sendToUpdateQueue(rc *request.AuthorizedContext, sub *models.OrgSub, payload *SubPayload) error {
	if err := s.UpdateQueue.Put(sub.ID, payload.PaymentGatewayCardToken, payload.SeatsCount); err != nil {
		return errors.Wrap(err, "failed to put to update subs queue")
	}
	_, err := s.finishQueueSending(rc, sub, models.OrgSubCommitStateUpdateInit, models.OrgSubCommitStateUpdateSentToQueue)
	return err
}

//nolint:gocyclo
func (s *basicService) Update(rc *request.AuthorizedContext, context *request.OrgSubID, payload *SubPayload) error {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID.OrgID).One(&org); err != nil {
		return errors.Wrap(err, "failed to get org from db")
	}
	if err := s.isAdminCached(rc, &org); err != nil {
		return errors.Wrap(err, "failed to check for admin")
	}

	var sub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).IDEq(context.SubID.SubID).One(&sub)
	if err == gorm.ErrRecordNotFound {
		return apierrors.ErrNotFound
	} else if err != nil {
		return errors.Wrap(err, "failed to fetch scoped org sub")
	}

	switch sub.CommitState {
	case models.OrgSubCommitStateCreateDone, models.OrgSubCommitStateUpdateDone:
		break // normal case
	case models.OrgSubCommitStateUpdateInit:
		rc.Log.Warnf("Reupdating sub with commit state %s, sending to queue: %#v",
			sub.CommitState, sub)
		return s.sendToUpdateQueue(rc, &sub, payload)
	case models.OrgSubCommitStateUpdateSentToQueue:
		rc.Log.Warnf("Reupdating sub with commit state %s, return ok: %#v",
			sub.CommitState, sub)
		return nil
	default:
		return fmt.Errorf("invalid sub commit state %s", sub.CommitState)
	}

	query := models.NewOrgSubQuerySet(rc.DB).
		IDEq(sub.ID).GetUpdater().
		SetCommitState(models.OrgSubCommitStateUpdateInit)

	n, err := query.UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update sub with id %d", sub.ID)
	}

	if n != 1 {
		return errors.New("no rows were updated, this really shouldn't happen")
	}

	return s.sendToUpdateQueue(rc, &sub, payload)
}

func (s basicService) sendToDeleteQueue(rc *request.AuthorizedContext, sub *models.OrgSub) error {
	if err := s.DeleteQueue.Put(sub.ID); err != nil {
		return errors.Wrap(err, "failed to put to delete subs queue")
	}
	_, err := s.finishQueueSending(rc, sub, models.OrgSubCommitStateDeleteInit, models.OrgSubCommitStateDeleteSentToQueue)
	return err
}

//nolint:gocyclo
func (s *basicService) Delete(rc *request.AuthorizedContext, context *request.OrgSubID) error {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID.OrgID).One(&org); err != nil {
		return errors.Wrap(err, "failed to get org from db")
	}
	if err := s.isAdminCached(rc, &org); err != nil {
		return errors.Wrap(err, "failed to check for admin")
	}

	var sub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).IDEq(context.SubID.SubID).One(&sub)
	if err == gorm.ErrRecordNotFound {
		return apierrors.ErrNotFound
	} else if err != nil {
		return errors.Wrap(err, "failed to fetch scoped org sub")
	}

	if sub.DeletedAt != nil {
		rc.Log.Warnf("Sub is already deleted")
		return nil
	}

	switch sub.CommitState {
	case models.OrgSubCommitStateCreateDone, models.OrgSubCommitStateUpdateDone:
		break // normal case: not being deleted now sub
	case models.OrgSubCommitStateDeleteInit:
		rc.Log.Warnf("Redeleting sub with commit state %s, sending to queue: %#v",
			sub.CommitState, sub)
		return s.sendToDeleteQueue(rc, &sub)
	case models.OrgSubCommitStateDeleteSentToQueue:
		rc.Log.Warnf("Redeleting sub with commit state %s, return ok: %#v",
			sub.CommitState, sub)
		return nil
	default:
		return fmt.Errorf("invalid sub commit state %s", sub.CommitState)
	}

	query := models.NewOrgSubQuerySet(rc.DB).
		IDEq(sub.ID).GetUpdater().
		SetCommitState(models.OrgSubCommitStateDeleteInit)

	n, err := query.UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to delete sub with id %d", sub.ID)
	}

	if n != 1 {
		return errors.New("no rows were updated, this really shouldn't happen")
	}

	return s.sendToDeleteQueue(rc, &sub)
}

func (s *basicService) EventCreate(rc *request.AnonymousContext, context *EventRequestContext, payload *EventRequestPayload) error {
	switch context.Provider {
	case implementations.SecurionPayProviderName:
	default:
		return errors.New("unexpected provider")
	}
	if payload.ID == "" {
		return errors.New("expected id")
	}
	if err := s.EventCreateQueue.Put(context.Provider, payload.ID); err != nil {
		return errors.Wrap(err, "failed to put to create payment event queue")
	}
	return nil
}

//TODO: This is a lot of code duplication between org and sub services,
// Ought to figure out a better way to handle this,
// but this method depends on cache and config which are only in services(?)

//nolint:dupl
func (s basicService) isAdminCached(rc *request.AuthorizedContext, org *models.Org) error {
	if org.ProviderPersonalUserID != 0 {
		if rc.Auth.ProviderUserID != uint64(org.ProviderPersonalUserID) {
			return errors.New("this is a personal org and this user doesn't own it")
		}
	} else {
		provider, err := s.ProviderFactory.Build(rc.Auth)
		if err != nil {
			return errors.Wrap(err, "failed to build provider")
		}

		if provider.Name() != org.Provider {
			return errors.Wrapf(err, "auth provider %s != request org provider %s", provider.Name(), org.Provider)
		}

		org, err := s.fetchProviderOrgCached(rc, false, provider, org.ProviderID)

		if err != nil {
			return errors.Wrap(err, "failed to fetch org from provider")
		}

		if !org.IsAdmin {
			return errors.New("no admin permission on org")
		}
	}
	return nil
}

func (s basicService) fetchProviderOrgCached(rc *request.AuthorizedContext, useCache bool, p provider.Provider, oid int) (*provider.Org, error) {
	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&org_id=%d&v=1", p.Name(), rc.Auth.UserID, oid)

	var org *provider.Org
	if useCache {
		if err := s.Cache.Get(key, &org); err != nil {
			rc.Log.Warnf("Can't fetch org from cache by key %s: %s", key, err)
			return s.fetchProviderOrgFromProvider(rc, p, oid)
		}

		if org != nil {
			rc.Log.Infof("Returning org(%d) from cache", org.ID)
			return org, nil
		}

		rc.Log.Infof("No org in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup org in cache, refreshing org from provider...")
	}

	var err error
	org, err = s.fetchProviderOrgFromProvider(rc, p, oid)
	if err != nil {
		return nil, err
	}

	cacheTTL := s.Cfg.GetDuration("ORG_CACHE_TTL", time.Hour*24)
	if err = s.Cache.Set(key, cacheTTL, org); err != nil {
		rc.Log.Warnf("Can't save org to cache by key %s: %s", key, err)
	}

	return org, nil
}

func (s basicService) fetchProviderOrgFromProvider(rc *request.AuthorizedContext, p provider.Provider, oid int) (*provider.Org, error) {
	org, err := p.GetOrgByID(rc.Ctx, oid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from provider by id %d", oid)
	}

	return org, nil
}
