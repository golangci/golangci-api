package paymentproviders

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/implementations/paddle"
	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/api/paymentproviders/implementations"
	"github.com/golangci/golangci-api/internal/api/paymentproviders/paymentprovider"
	"github.com/golangci/golangci-api/internal/shared/logutil"
)

type Factory interface {
	Build(provider string) (paymentprovider.Provider, error)
}

type basicFactory struct {
	log logutil.Log
	cfg config.Config
}

func NewBasicFactory(log logutil.Log, cfg config.Config) Factory {
	return &basicFactory{
		log: log,
		cfg: cfg,
	}
}

func (f basicFactory) buildImpl(provider string) (paymentprovider.Provider, error) {
	switch provider {
	case implementations.SecurionPayProviderName:
		return implementations.NewSecurionPay(f.log), nil
	case paddle.ProviderName:
		return paddle.NewProvider(f.log, f.cfg)
	default:
		return nil, fmt.Errorf("invalid provider name %q", provider)
	}

}

func (f *basicFactory) Build(provider string) (paymentprovider.Provider, error) {
	p, err := f.buildImpl(provider)
	if err != nil {
		return nil, err
	}

	return implementations.NewStableProvider(p, time.Second*30, 3), nil
}
