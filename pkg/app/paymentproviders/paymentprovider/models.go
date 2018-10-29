package paymentprovider

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

type UpdatePayload struct {
	CardToken  string
	SeatsCount int
}
