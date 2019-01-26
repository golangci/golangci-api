package paddle

import (
	"encoding/json"

	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"

	"github.com/pkg/errors"
)

type commonEvent struct {
	AlertName string `schema:"alert_name"`
	AlertID   string `schema:"alert_id"`    // It's the int, but use it as string because we save event id as string to db
	Signature string `schema:"p_signature"` // TODO(d.isaev): use it in addition to secret token in URL
	EventTime string `schema:"event_time"`

	Email            string
	MarketingConsent string `schema:"marketing_consent"`
}

func (e commonEvent) GetID() string {
	return e.AlertID
}

func (e commonEvent) GetType() string {
	return e.AlertName
}

type eventWithoutUser struct{}

func (e eventWithoutUser) GetUserID() (*uint, error) {
	return nil, nil
}

type eventWithID interface {
	GetType() string
	GetID() string
	GetUserID() (*uint, error)
}

type urlsData struct {
	UpdateURL string `schema:"update_url"`
	CancelURL string `schema:"cancel_url"`
}

type subCreatedEvent struct {
	commonEvent
	eventWithPassthrough
	subscriptionOperation
	urlsData

	Currency     string
	NextBillDate string `schema:"next_bill_date"`
}

type passthroughData struct {
	UserID               uint
	OrgProvider, OrgName string
}

type eventWithPassthrough struct {
	Passthrough string
}

func (e eventWithPassthrough) GetUserID() (*uint, error) {
	var pd passthroughData
	if err := json.Unmarshal([]byte(e.Passthrough), &pd); err != nil {
		return nil, errors.Wrap(err, "failed to json unmarshal passthrough")
	}

	return &pd.UserID, nil
}

func (e eventWithPassthrough) GetOrgID(db *gorm.DB) (uint, error) {
	var pd passthroughData
	if err := json.Unmarshal([]byte(e.Passthrough), &pd); err != nil {
		return 0, errors.Wrap(err, "failed to json unmarshal passthrough")
	}

	var org models.Org
	if err := models.NewOrgQuerySet(db).ProviderEq(pd.OrgProvider).NameEq(pd.OrgName).One(&org); err != nil {
		return 0, errors.Wrapf(err, "failed to get org %s/%s", pd.OrgProvider, pd.OrgName)
	}

	return org.ID, nil
}

type newAudienceEvent struct {
	commonEvent
	eventWithoutUser

	CreatedAt  string `schema:"created_at"`
	UpdatedAt  string `schema:"updated_at"`
	Products   string
	Source     string
	UserID     string `schema:"user_id"`
	Subscribed string // no in documentation
}

type subscriptionStatus struct {
	SubscriptionID     int64 `schema:"subscription_id"`
	SubscriptionPlanID int64 `schema:"subscription_plan_id"`
	Status             string
	CheckoutID         string `schema:"checkout_id"`
}

type subscriptionOperation struct {
	subscriptionStatus

	Quantity  int
	UnitPrice string `schema:"unit_price"`
}

//nolint:megacheck
type balanceData struct {
	BalanceCurrency string `schema:"balance_currency"`
	BalanceEarnings string `schema:"balance_earnings"`
	BalanceTax      string `schema:"balance_tax"`
	BalanceFee      string `schema:"balance_fee"`
	BalanceGross    string `schema:"balance_gross"`
}

//nolint:megacheck
type subPaymentSucceededEvent struct {
	commonEvent
	eventWithPassthrough
	subscriptionOperation
	balanceData

	Currency       string
	NextBillDate   string `schema:"next_bill_date"`
	OrderID        string `schema:"order_id"`
	Country        string
	SaleGross      string `schema:"sale_gross"`
	Fee            string
	Earnings       string
	CustomerName   string `schema:"customer_name"`
	UserID         string `schema:"user_id"`
	PlanName       string `schema:"plan_name"`
	PaymentTax     string `schema:"payment_tax"`
	PaymentMethod  string `schema:"payment_method"`
	Coupon         string
	ReceiptURL     string `schema:"receipt_url"`
	InitialPayment bool   `schema:"initial_payment"`
	Instalments    int
}

//nolint:megacheck
type subPaymentFailedEvent struct {
	commonEvent
	eventWithPassthrough
	subscriptionOperation
	urlsData

	Currency      string
	NextRetryDate string `schema:"next_retry_date"`
	Amount        string
	HardFailure   bool `schema:"hard_failure"`
}

type subCancelledEvent struct {
	commonEvent
	subscriptionOperation
	eventWithPassthrough

	Currency                  string
	CancellationEffectiveDate string `schema:"cancellation_effective_date"`
	UserID                    string `schema:"user_id"`
}

//nolint:megacheck
type subUpdatedEvent struct {
	commonEvent
	subscriptionStatus
	eventWithPassthrough
	urlsData

	NextBillDate string `schema:"next_bill_date"`
	NewQuantity  int    `schema:"new_quantity"`
	OldQuantity  int    `schema:"old_quantity"`
	NewUnitPrice string `schema:"new_unit_price"`
	OldUnitPrice string `schema:"old_unit_price"`
	NewPrice     string `schema:"new_price"`
	OldPrice     string `schema:"old_price"`
}
