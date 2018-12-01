package implementations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/paymentprovider"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/pkg/errors"
)

const SecurionPayProviderName = "securionpay"

type securionPay struct {
	APIRoot, APISecret, PlanID string
	client                     http.Client
	log                        logutil.Log
}

var initCachedSecurionPay sync.Once
var cachedSecurionPay paymentprovider.Provider

func NewSecurionPay(log logutil.Log) paymentprovider.Provider {
	initCachedSecurionPay.Do(func() {
		cachedSecurionPay = &securionPay{
			APIRoot:   "https://api.securionpay.com",
			APISecret: os.Getenv("SECURIONPAY_SECRET"),
			// TODO: Hard coded for now, consider making it a parameter in create sub function...
			PlanID: os.Getenv("SECURIONPAY_PLANID"),

			client: http.Client{},
			log:    log,
		}
	})
	return cachedSecurionPay
}

func (s *securionPay) Name() string {
	return SecurionPayProviderName
}

func (s *securionPay) SetBaseURL(u string) error {
	_, err := url.Parse(u)
	if err != nil {
		return errors.Wrap(err, "failed to parse url")
	}

	s.APIRoot = u
	return nil
}

func (s *securionPay) CreateCustomer(ctx context.Context, email string, token string) (*paymentprovider.Customer, error) {
	form := url.Values{}
	form.Add("email", email)
	form.Add("card", token)
	req, err := http.NewRequest("POST", s.APIRoot+"/customers", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request for /customers")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request for /customers")
	}
	var temp struct {
		ID    string `json:"id"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrap(err, "failed to decode response for /customers")
	}
	if temp.Error != nil {
		switch temp.Error.Type {
		case "invalid_request":
			return nil, paymentprovider.ErrInvalidCardToken
		default:
			err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
			return nil, errors.Wrap(err, "request to /customers failed")
		}
	}

	return &paymentprovider.Customer{ID: temp.ID}, nil
}

func (s *securionPay) GetSubscription(ctx context.Context, cust string, sub string) (*paymentprovider.Subscription, error) {
	path := fmt.Sprintf("/customers/%s/subscriptions/%s", cust, sub)
	req, err := http.NewRequest("GET", s.APIRoot+path, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return nil, errors.Wrapf(err, "request to %s failed", path)
	}

	return &paymentprovider.Subscription{ID: temp.ID, Status: paymentprovider.SubscriptionStatus(temp.Status)}, nil
}

func (s *securionPay) GetSubscriptions(ctx context.Context, cust string) ([]paymentprovider.Subscription, error) {
	path := fmt.Sprintf("/customers/%s/subscriptions", cust)
	u, err := url.Parse(s.APIRoot + path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create URL for %s", path)
	}
	q := u.Query()
	q.Set("limit", "100")
	u.RawQuery = q.Encode()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		List []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"list,omitempty"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return nil, errors.Wrapf(err, "request to %s failed", path)
	}

	var ret []paymentprovider.Subscription
	for _, s := range temp.List {
		ret = append(ret, paymentprovider.Subscription{
			ID:     s.ID,
			Status: paymentprovider.SubscriptionStatus(s.Status),
		})
	}
	return ret, nil
}

func (s *securionPay) CreateSubscription(ctx context.Context, cust string, seats int) (*paymentprovider.Subscription, error) {
	form := url.Values{}
	form.Add("planId", s.PlanID)
	if seats > 0 {
		form.Add("quantity", strconv.Itoa(seats))
	}
	path := fmt.Sprintf("/customers/%s/subscriptions", cust)
	req, err := http.NewRequest("POST", s.APIRoot+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return nil, errors.Wrapf(err, "request to %s failed", path)
	}

	return &paymentprovider.Subscription{ID: temp.ID, Status: paymentprovider.SubscriptionStatus(temp.Status)}, nil
}

func (s *securionPay) UpdateSubscription(ctx context.Context, cust string, sub string, payload paymentprovider.SubscriptionUpdatePayload) (*paymentprovider.Subscription, error) {
	form := url.Values{}
	if payload.SeatsCount > 0 {
		form.Add("quantity", strconv.Itoa(payload.SeatsCount))
	}
	if payload.CardToken != "" {
		form.Add("card", payload.CardToken)
	}
	path := fmt.Sprintf("/customers/%s/subscriptions/%s", cust, sub)
	req, err := http.NewRequest("POST", s.APIRoot+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return nil, errors.Wrapf(err, "request to %s failed", path)
	}

	return &paymentprovider.Subscription{ID: temp.ID, Status: paymentprovider.SubscriptionStatus(temp.Status)}, nil
}

func (s *securionPay) DeleteSubscription(ctx context.Context, cust string, sub string) error {
	path := fmt.Sprintf("/customers/%s/subscriptions/%s", cust, sub)
	req, err := http.NewRequest("DELETE", s.APIRoot+path, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return errors.Wrapf(err, "request to %s failed", path)
	}
	return nil
}

func (s *securionPay) GetEvent(ctx context.Context, event string) (*paymentprovider.Event, error) {
	path := fmt.Sprintf("/events/%s", event)
	req, err := http.NewRequest("GET", s.APIRoot+path, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}
	req.SetBasicAuth(s.APISecret, "")
	resp, err := s.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute request for %s", path)
	}
	var temp struct {
		ID    string          `json:"id"`
		Type  string          `json:"type"`
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&temp); err != nil {
		return nil, errors.Wrapf(err, "failed to decode response for %s", path)
	}
	if temp.Error != nil {
		err := fmt.Errorf("%s: %s", temp.Error.Type, temp.Error.Message)
		return nil, errors.Wrapf(err, "request to %s failed", path)
	}

	return &paymentprovider.Event{ID: temp.ID, Type: temp.Type, Data: temp.Data}, nil
}
