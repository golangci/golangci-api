package subscription

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/internal/api/apierrors"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/implementations/paddle"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/golangci/golangci-api/pkg/api/returntypes"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/paymentevents"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/subs"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type UpdatePayload struct {
	SeatsCount int `json:"seatsCount"`
	OrgVersion int `json:"orgVersion"`
	Version    int `json:"version"`
}

func (p UpdatePayload) FillLogContext(lctx logutil.Context) {
	lctx["seats_count"] = p.SeatsCount
	lctx["version"] = p.Version
	lctx["org_version"] = p.OrgVersion
}

type EventRequestContext struct {
	Provider string `request:"provider,urlPart,"`
	Token    string `request:"token,urlPart,"`
}

func (r EventRequestContext) FillLogContext(lctx logutil.Context) {
	lctx["provider"] = r.Provider
}

type Service interface {
	//url:/v1/orgs/{provider}/{name}/subscription
	Get(rc *request.AuthorizedContext, reqOrg *request.Org) (*returntypes.SubInfo, error)

	//url:/v1/orgs/{provider}/{name}/subscription method:PUT
	Update(rc *request.AuthorizedContext, reqOrg *request.Org, payload *UpdatePayload) error

	//url:/v1/payments/{provider}/{token}/events method:POST
	EventCreate(rc *request.AnonymousContext, context *EventRequestContext, body request.Body) error
}

func NewBasicService(orgPolicy *policy.Organization, cfg config.Config,
	update *subs.UpdaterProducer, pec *paymentevents.CreatorProducer) *BasicService {

	return &BasicService{
		orgPolicy:        orgPolicy,
		cfg:              cfg,
		updateQueue:      update,
		eventCreateQueue: pec,
	}
}

type BasicService struct {
	orgPolicy *policy.Organization
	cfg       config.Config

	updateQueue      *subs.UpdaterProducer
	eventCreateQueue *paymentevents.CreatorProducer
}

func (s BasicService) calcPaddleTrialDaysAuth(days int) string {
	if days == 0 {
		return ""
	}

	secret := s.cfg.GetString("PADDLE_STANDARD_CHECKOUT_SECRET_KEY")
	input := fmt.Sprintf("%d%s", days, secret)
	sum := md5.Sum([]byte(input))
	return fmt.Sprintf("%x", sum)
}

func (s BasicService) subFromModel(sub models.OrgSub, trialDuration time.Duration) *returntypes.SubInfo {
	status := "active"
	if sub.IsCreating() {
		status = "creating"
	} else if sub.IsUpdating() {
		status = "updating"
	} else if sub.IsDeleting() {
		status = "deleting"
	}
	days := int(trialDuration.Hours() / 24)
	return &returntypes.SubInfo{
		SeatsCount:           sub.SeatsCount,
		PricePerSeat:         sub.PricePerSeat,
		Status:               status,
		Version:              sub.Version,
		CancelURL:            sub.CancelURL,
		TrialAllowanceInDays: days,
		PaddleTrialDaysAuth:  s.calcPaddleTrialDaysAuth(days),
	}
}

func (s BasicService) subInfoInactive(trialDuration time.Duration) *returntypes.SubInfo {
	days := int(trialDuration.Hours() / 24)
	return &returntypes.SubInfo{
		SeatsCount:           0,
		Status:               "inactive",
		Version:              0,
		PricePerSeat:         "0",
		CancelURL:            "",
		TrialAllowanceInDays: days,
		PaddleTrialDaysAuth:  s.calcPaddleTrialDaysAuth(days),
	}
}

// Find existing subscription for the organization and return it in "subscription" field (if subscription exists).
// If no subscription return trial duration: defend from always recreating subscription with the trial: select created_at
// of first created subscription and get trial duration as (config.getTrialDuration() - (now() -
// first_subscription.created_at).
func (s BasicService) Get(rc *request.AuthorizedContext, reqOrg *request.Org) (*returntypes.SubInfo, error) {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).NameEq(reqOrg.Name).ProviderEq(reqOrg.Provider).One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to get org from db")
	}
	if err := s.orgPolicy.CheckCanModify(rc, &org); err != nil {
		// TODO: allow to view org settings and subscription but not to update
		if err == policy.ErrNotOrgAdmin {
			err = policy.ErrNotOrgAdmin.WithMessage("Only organization admins can view organization settings and subscription")
		}
		if err == policy.ErrNotOrgMember {
			err = policy.ErrNotOrgMember.WithMessage("Only organization members can view organization settings and subscription")
		}
		return nil, errors.Wrap(err, "failed to check for admin")
	}

	trialPeriod := s.cfg.GetDuration("SUB_TRIAL_PERIOD", time.Hour*24*30)

	var sub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).OrgIDEq(org.ID).One(&sub)
	if err == gorm.ErrRecordNotFound {
		// No sub, try to get oldest probably deleted sub and use that in math for trial.
		err = models.NewOrgSubQuerySet(rc.DB.Unscoped()).OrgIDEq(org.ID).OrderAscByCreatedAt().One(&sub)
		if err == gorm.ErrRecordNotFound {
			return s.subInfoInactive(trialPeriod), nil
		} else if err != nil {
			return nil, errors.Wrap(err, "failed to fetch oldest unscoped org sub")
		}

		elapsedFromFirstSub := time.Since(sub.CreatedAt)
		if elapsedFromFirstSub > trialPeriod {
			trialPeriod = 0
		} else {
			trialPeriod -= elapsedFromFirstSub
		}
		return s.subInfoInactive(trialPeriod), nil
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to fetch scoped org sub")
	}

	return s.subFromModel(sub, trialPeriod), nil
}

func (s BasicService) finishQueueSending(rc *request.AuthorizedContext, sub *models.OrgSub,
	expState, setState models.OrgSubCommitState) error {

	err := models.NewOrgSubQuerySet(rc.DB).
		IDEq(sub.ID).CommitStateEq(expState).
		GetUpdater().
		SetCommitState(setState).
		Update()
	if err != nil {
		return errors.Wrapf(err, "failed to update commit state to %s for sub with id %d",
			setState, sub.ID)
	}
	sub.CommitState = setState

	return nil
}

func (s BasicService) sendToUpdateQueue(rc *request.AuthorizedContext, sub *models.OrgSub, payload *UpdatePayload) error {
	if err := s.updateQueue.Put(sub.ID, payload.SeatsCount); err != nil {
		return errors.Wrap(err, "failed to put to update subs queue")
	}

	return s.finishQueueSending(rc, sub, models.OrgSubCommitStateUpdateInit, models.OrgSubCommitStateUpdateSentToQueue)
}

func (s BasicService) getSubForUpdate(rc *request.AuthorizedContext,
	reqOrg *request.Org, payload *UpdatePayload) (*models.OrgSub, error) {

	var org models.Org
	qs := models.NewOrgQuerySet(rc.DB).NameEq(reqOrg.Name).ProviderEq(reqOrg.Provider)
	if err := qs.One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to get org from db")
	}
	if org.Version != payload.OrgVersion {
		return nil, apierrors.NewRaceConditionError("organization settings were changed in parallel")
	}

	if err := s.orgPolicy.CheckCanModify(rc, &org); err != nil {
		if err == policy.ErrNotOrgAdmin {
			err = policy.ErrNotOrgAdmin.WithMessage("Only organization admins can update subscription")
		}
		if err == policy.ErrNotOrgMember {
			err = policy.ErrNotOrgMember.WithMessage("Only organization members can update subscription")
		}
		return nil, errors.Wrap(err, "failed to check for admin")
	}

	var sub models.OrgSub
	if err := models.NewOrgSubQuerySet(rc.DB).OrgIDEq(org.ID).One(&sub); err != nil {
		return nil, errors.Wrap(err, "failed to fetch scoped org sub")
	}

	if sub.Version != payload.Version {
		return nil, apierrors.NewRaceConditionError("subscription was changed in parallel")
	}

	return &sub, nil
}

func (s BasicService) Update(rc *request.AuthorizedContext, reqOrg *request.Org, payload *UpdatePayload) error {
	sub, err := s.getSubForUpdate(rc, reqOrg, payload)
	if err != nil {
		return errors.Wrap(err, "failed to get subscription for updating")
	}

	switch sub.CommitState {
	case models.OrgSubCommitStateCreateDone, models.OrgSubCommitStateUpdateDone:
		break // normal case
	case models.OrgSubCommitStateUpdateInit:
		rc.Log.Warnf("Reupdating sub with commit state %s, sending to queue: %#v",
			sub.CommitState, sub)
		return s.sendToUpdateQueue(rc, sub, payload)
	case models.OrgSubCommitStateUpdateSentToQueue:
		rc.Log.Warnf("Reupdating sub with commit state %s, return ok: %#v",
			sub.CommitState, sub)
		return nil
	default:
		return fmt.Errorf("invalid sub commit state %s", sub.CommitState)
	}

	query := models.NewOrgSubQuerySet(rc.DB).
		IDEq(sub.ID).
		CommitStateEq(sub.CommitState).
		VersionEq(sub.Version).
		GetUpdater().
		SetCommitState(models.OrgSubCommitStateUpdateInit).
		SetVersion(sub.Version + 1)

	if err = query.UpdateRequired(); err != nil {
		return errors.Wrapf(err, "failed to update sub with id %d", sub.ID)
	}
	sub.Version++

	return s.sendToUpdateQueue(rc, sub, payload)
}

func (s BasicService) EventCreate(rc *request.AnonymousContext, context *EventRequestContext, body request.Body) error {
	switch context.Provider {
	case paddle.ProviderName:
	default:
		return errors.New("unexpected provider")
	}

	cfgTokenKey := fmt.Sprintf("%s_token", context.Provider)
	cfgToken := s.cfg.GetString(cfgTokenKey)
	if cfgToken == "" {
		return errors.New("no token in config")
	}

	if context.Token != cfgToken {
		rc.Log.Errorf("Invalid token: %s (got) != %s (in config)", context.Token, cfgToken)
		return errors.New("invalid token")
	}

	const maxBodySize = 256 * 1024 // AWS SQS message limit
	if len(body) > maxBodySize {
		return fmt.Errorf("too big body of len %d", len(body))
	}

	if err := s.eventCreateQueue.Put(context.Provider, string(body)); err != nil {
		return errors.Wrap(err, "failed to put to create payment event queue")
	}

	return nil
}
