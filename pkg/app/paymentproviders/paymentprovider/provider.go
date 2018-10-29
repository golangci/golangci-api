package provider

import "context"

type Provider interface {
	Name() string

	CreateCustomer(ctx context.Context, email string, token string) (*Customer, error)
	UpdateCustomer(ctx context.Context, cust string, payload CustomerUpdatePayload) (*Customer, error)
	DeleteSubscription(cx context.Context, cust string) error

	GetSubscription(ctx context.Context, cust string, sub string) (*Subscription, error)
	GetSubscriptions(ctx context.Context, cust string) ([]Subscription, error)
	CreateSubscription(ctx context.Context, cust string, seats int) (*Subscription, error)
	UpdateSubscription(ctx context.Context, cust string, sub string, payload SubscriptionUpdatePayload) (*Subscription, error)
	DeleteSubscription(ctx context.Context, cust string, sub string) error
}
