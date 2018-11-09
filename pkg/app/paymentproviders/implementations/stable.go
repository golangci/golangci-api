package implementations

import (
	"context"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/golangci/golangci-api/pkg/app/paymentproviders/paymentprovider"
)

type stableProvider struct {
	underlying   paymentprovider.Provider
	totalTimeout time.Duration
	maxRetries   int
}

func NewStableProvider(underlying paymentprovider.Provider, totalTimeout time.Duration, maxRetries int) paymentprovider.Provider {
	return &stableProvider{
		underlying:   underlying,
		totalTimeout: totalTimeout,
		maxRetries:   maxRetries,
	}
}

func (p stableProvider) retry(f func() error) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = p.totalTimeout

	bmr := backoff.WithMaxRetries(b, uint64(p.maxRetries))
	if err := backoff.Retry(f, bmr); err != nil {
		return err
	}

	return nil
}

func (p *stableProvider) Name() string {
	return p.underlying.Name()
}

func (p *stableProvider) SetBaseURL(s string) error {
	return p.underlying.SetBaseURL(s)
}

func (p *stableProvider) CreateCustomer(ctx context.Context, email string, token string) (retCust *paymentprovider.Customer, err error) {
	_ = p.retry(func() error {
		retCust, err = p.underlying.CreateCustomer(ctx, email, token)
		return err
	})
	return
}

func (p *stableProvider) GetSubscription(ctx context.Context, cust string, sub string) (retSub *paymentprovider.Subscription, err error) {
	_ = p.retry(func() error {
		retSub, err = p.underlying.GetSubscription(ctx, cust, sub)
		return err
	})
	return
}

func (p *stableProvider) GetSubscriptions(ctx context.Context, cust string) (retSubs []paymentprovider.Subscription, err error) {
	_ = p.retry(func() error {
		retSubs, err = p.underlying.GetSubscriptions(ctx, cust)
		return err
	})
	return
}

func (p *stableProvider) CreateSubscription(ctx context.Context, cust string, seats int) (retSub *paymentprovider.Subscription, err error) {
	_ = p.retry(func() error {
		retSub, err = p.underlying.CreateSubscription(ctx, cust, seats)
		return err
	})
	return
}

func (p *stableProvider) UpdateSubscription(ctx context.Context, cust string, sub string, payload paymentprovider.SubscriptionUpdatePayload) (retSub *paymentprovider.Subscription, err error) {
	_ = p.retry(func() error {
		retSub, err = p.underlying.UpdateSubscription(ctx, cust, sub, payload)
		return err
	})
	return
}

func (p *stableProvider) DeleteSubscription(ctx context.Context, cust string, sub string) (err error) {
	_ = p.retry(func() error {
		err = p.underlying.DeleteSubscription(ctx, cust, sub)
		return err
	})
	return
}

func (p *stableProvider) GetEvent(ctx context.Context, event string) (retEvent *paymentprovider.Event, err error) {
	_ = p.retry(func() error {
		retEvent, err = p.underlying.GetEvent(ctx, event)
		return err
	})
	return
}
