package mocks

import (
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
)

type ProviderTransformer func(p provider.Provider) provider.Provider

type ProviderFactory struct {
	orig        providers.Factory
	transformer ProviderTransformer
}

var _ providers.Factory = &ProviderFactory{}

func NewProviderFactory(transformer ProviderTransformer, orig providers.Factory) *ProviderFactory {
	return &ProviderFactory{
		orig:        orig,
		transformer: transformer,
	}
}

func (f ProviderFactory) Build(auth *models.Auth) (provider.Provider, error) {
	p, err := f.orig.Build(auth)
	if p != nil {
		p = f.transformer(p)
	}
	return p, err
}

func (f ProviderFactory) BuildForUser(db *gorm.DB, userID uint) (provider.Provider, error) {
	p, err := f.orig.BuildForUser(db, userID)
	if p != nil {
		p = f.transformer(p)
	}
	return p, err
}

func (f ProviderFactory) BuildForToken(providerName, accessToken string) (provider.Provider, error) {
	p, err := f.orig.BuildForToken(providerName, accessToken)
	if p != nil {
		p = f.transformer(p)
	}
	return p, err
}
