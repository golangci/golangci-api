package paymentprovider

import "encoding/json"

type SubscriptionStatus string

const (
	SubscriptionStatusTrialing  SubscriptionStatus = "trialing"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusUnpaid    SubscriptionStatus = "unpaid"
)

type Customer struct {
	ID    string
	Email string
}

type Subscription struct {
	ID     string
	Status SubscriptionStatus
}

type Event struct {
	ID   string
	Type string
	Data json.RawMessage
}

type SubscriptionUpdatePayload struct {
	CardToken  string
	SeatsCount int
}

type CustomerUpdatePayload struct {
}
