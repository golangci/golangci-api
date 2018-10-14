package mocks

import (
	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
	"github.com/jinzhu/gorm"
)

type ProviderTransformer func(p provider.Provider) provider.Provider

type ProviderFactory struct {
	orig        providers.Factory
	transformer ProviderTransformer
}

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
