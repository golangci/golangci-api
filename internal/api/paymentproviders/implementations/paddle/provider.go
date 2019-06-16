package paddle

import (
	"context"
	"fmt"
	"net/url"

	"github.com/levigross/grequests"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/paymentprovider"
	"github.com/golangci/golangci-api/internal/shared/logutil"
)

type Provider struct {
	log logutil.Log

	vendorID       int
	vendorAuthCode string

	apiRoot string
}

func NewProvider(log logutil.Log, cfg config.Config) (*Provider, error) {
	vendorID := cfg.GetInt("PADDLE_VENDOR_ID", 0)
	if vendorID == 0 {
		return nil, errors.New("no paddle vendor id")
	}

	vendorAuthCode := cfg.GetString("PADDLE_VENDOR_AUTH_CODE")
	if vendorAuthCode == "" {
		return nil, errors.New("no paddle vendor auth code")
	}

	return &Provider{
		log:            log,
		vendorID:       vendorID,
		vendorAuthCode: vendorAuthCode,
		apiRoot:        "https://vendors.paddle.com",
	}, nil
}

func (p Provider) getRequestAuth() requestAuth {
	return requestAuth{
		VendorID:       p.vendorID,
		VendorAuthCode: p.vendorAuthCode,
	}
}

func (p Provider) Name() string {
	return ProviderName
}

func (p Provider) SetBaseURL(u string) error {
	_, err := url.Parse(u)
	if err != nil {
		return errors.Wrap(err, "failed to parse url")
	}

	p.apiRoot = u
	return nil
}

func (p Provider) CreateCustomer(ctx context.Context, email string, token string) (*paymentprovider.Customer, error) {
	panic("not implemented")
}

func (p Provider) GetSubscription(ctx context.Context, cust string, sub string) (*paymentprovider.Subscription, error) {
	panic("not implemented")
}

func (p Provider) GetSubscriptions(ctx context.Context, cust string) ([]paymentprovider.Subscription, error) {
	panic("not implemented")
}

func (p Provider) CreateSubscription(ctx context.Context, cust string, seats int) (*paymentprovider.Subscription, error) {
	panic("not implemented")
}

func (p Provider) UpdateSubscription(ctx context.Context, cust string, sub string,
	payload paymentprovider.SubscriptionUpdatePayload) (*paymentprovider.Subscription, error) {

	apiURL := fmt.Sprintf("%s/api/2.0/subscription/users/update", p.apiRoot)

	type updateSubReq struct {
		requestAuth
		SubscriptionID string `schema:"subscription_id"`
		Quantity       int    `schema:"quantity"`
	}

	req := updateSubReq{
		requestAuth:    p.getRequestAuth(),
		SubscriptionID: sub,
		Quantity:       payload.SeatsCount,
	}

	data, err := structToGrequestsData(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request data")
	}
	resp, err := grequests.Post(apiURL, &grequests.RequestOptions{
		Data: data,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to make subscription update request")
	}

	if !resp.Ok {
		return nil, fmt.Errorf("provider response code %d", resp.StatusCode)
	}

	p.log.Infof("Update subscription request %#v to paddle response: %s", req, apiURL, resp.String())

	var respData struct {
		Success bool
		Error   struct {
			Code    int
			Message string
		}
	}

	if err = resp.JSON(&respData); err != nil {
		return nil, errors.Wrap(err, "failed to decode response json")
	}
	if !respData.Success {
		return nil, fmt.Errorf("error from paddle: %s", respData.Error.Message)
	}

	return &paymentprovider.Subscription{
		ID:     sub,
		Status: "", // no in paddle response
	}, nil
}

func (p Provider) DeleteSubscription(ctx context.Context, cust string, sub string) error {
	panic("not implemented")
}

func (p Provider) GetEvent(ctx context.Context, eventID string) (*paymentprovider.Event, error) {
	panic("not implemented")
}
