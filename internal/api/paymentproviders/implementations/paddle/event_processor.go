package paddle

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/gorilla/schema"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type EventProcessor struct {
	Tx  *gorm.DB
	Log logutil.Log
}

const (
	eventSubCreated                 = "subscription_created"
	eventSubPaymentSucceeded        = "subscription_payment_succeeded"
	eventSubPaymentFailed           = "subscription_payment_failed"
	eventSubCancelled               = "subscription_cancelled"
	eventSubUpdated                 = "subscription_updated"
	eventNewAudienceMember          = "new_audience_member"
	eventHighRiskTransactionCreated = "high_risk_transaction_created"
)

//nolint:gocyclo
func (ep EventProcessor) parseEvent(payload string) (eventWithID, error) {
	vs, err := url.ParseQuery(payload)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse body of len %d", len(payload))
	}

	eventType := vs.Get("alert_name")
	ep.Log.Infof("Got paddle event of type %s: %s", eventType, payload)

	var evWithID eventWithID
	switch eventType {
	case "":
		return nil, errors.New("no alert_name key")
	case eventSubCreated:
		evWithID = &subCreatedEvent{}
	case eventSubPaymentSucceeded:
		evWithID = &subPaymentSucceededEvent{}
	case eventSubPaymentFailed:
		evWithID = &subPaymentFailedEvent{}
	case eventSubCancelled:
		evWithID = &subCancelledEvent{}
	case eventSubUpdated:
		evWithID = &subUpdatedEvent{}
	case eventNewAudienceMember:
		evWithID = &newAudienceEvent{}
	case eventHighRiskTransactionCreated:
		evWithID = &highRiskTransactionCreatedEvent{}
	default:
		return nil, fmt.Errorf("got unknown event type %s", eventType)
	}

	formDecoder := schema.NewDecoder()
	formDecoder.IgnoreUnknownKeys(true) // Paddle changes API format too often
	if err = formDecoder.Decode(evWithID, vs); err != nil {
		return nil, errors.Wrapf(err, "failed to decode %s event", eventType)
	}
	ep.Log.Infof("Parsed %s event: %#v", eventType, evWithID)

	return evWithID, nil
}

func (ep EventProcessor) processSubCreatedEvent(ev *subCreatedEvent, eventUUID string) error {
	orgID, err := ev.GetOrgID(ep.Tx)
	if err != nil {
		return errors.Wrap(err, "failed to get org id for event")
	}

	userID, err := ev.GetUserID()
	if err != nil {
		return errors.Wrap(err, "failed to get user id for event")
	}

	dbSub := models.OrgSub{
		PaymentGatewayName:           ProviderName,
		PaymentGatewayCardToken:      "",
		PaymentGatewayCustomerID:     "",
		PaymentGatewaySubscriptionID: strconv.FormatInt(ev.SubscriptionID, 10),
		BillingUserID:                *userID, // TODO: remove
		OrgID:                        orgID,
		SeatsCount:                   ev.Quantity,
		PricePerSeat:                 ev.UnitPrice,
		CommitState:                  models.OrgSubCommitStateCreateDone,
		IdempotencyKey:               eventUUID,
		Version:                      0,
		CancelURL:                    ev.CancelURL,
	}
	if err = dbSub.Create(ep.Tx); err != nil {
		return errors.Wrap(err, "failed to save subscription to db")
	}

	return nil
}

func (ep EventProcessor) processSubCancelledEvent(ev *subCancelledEvent) error {
	var sub models.OrgSub
	providerSubID := strconv.FormatInt(ev.SubscriptionID, 10)
	qs := models.NewOrgSubQuerySet(ep.Tx).PaymentGatewaySubscriptionIDEq(providerSubID)
	if err := qs.One(&sub); err != nil {
		return errors.Wrapf(err, "failed to fetch sub with payment provider id %s", providerSubID)
	}

	if err := sub.Delete(ep.Tx); err != nil {
		return errors.Wrapf(err, "failed to delete subscription id %d", sub.ID)
	}

	ep.Log.Infof("Deleted subscription %d on cancelled event", sub.ID)
	return nil
}

func (ep EventProcessor) saveEvent(ev eventWithID) error {
	userID, err := ev.GetUserID()
	if err != nil {
		return errors.Wrap(err, "failed to get user id for event")
	}

	payloadJSON, err := json.Marshal(ev)
	if err != nil {
		return errors.Wrap(err, "failed to marshal json")
	}

	dbEvent := models.PaymentGatewayEvent{
		Provider:   ProviderName,
		ProviderID: ev.GetID(),
		UserID:     userID,
		Type:       ev.GetType(),
		Data:       payloadJSON,
	}
	if err := dbEvent.Create(ep.Tx); err != nil {
		return errors.Wrap(err, "failed to save event to db")
	}

	return nil
}

func (ep EventProcessor) Process(payload string, eventUUID string) error {
	evWithID, err := ep.parseEvent(payload)
	if err != nil {
		return errors.Wrap(err, "failed to parse event")
	}

	var existingEv models.PaymentGatewayEvent
	err = models.NewPaymentGatewayEventQuerySet(ep.Tx).ProviderIDEq(evWithID.GetID()).One(&existingEv)
	if err != gorm.ErrRecordNotFound {
		ep.Log.Infof("Event with id %s was already processed: it exists in db", evWithID.GetID())
		return nil
	}

	switch evWithID.GetType() {
	case eventSubCreated:
		if err = ep.processSubCreatedEvent(evWithID.(*subCreatedEvent), eventUUID); err != nil {
			return errors.Wrapf(err, "failed to process %s event", evWithID.GetType())
		}
	case eventSubCancelled:
		if err = ep.processSubCancelledEvent(evWithID.(*subCancelledEvent)); err != nil {
			return errors.Wrapf(err, "failed to process %s event", evWithID.GetType())
		}
	}

	return ep.saveEvent(evWithID)
}
