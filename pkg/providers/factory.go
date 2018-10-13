package providers

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/app/hooks"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/providers/implementations"
	"github.com/golangci/golangci-api/pkg/providers/provider"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Factory interface {
	Build(auth *models.Auth) (provider.Provider, error)
	BuildForUser(db *gorm.DB, userID uint) (provider.Provider, error)
}

type BasicFactory struct {
	hooksInjector *hooks.Injector
	log           logutil.Log
}

func NewBasicFactory(hooksInjector *hooks.Injector, log logutil.Log) *BasicFactory {
	return &BasicFactory{
		hooksInjector: hooksInjector,
		log:           log,
	}
}

func (f BasicFactory) buildImpl(auth *models.Auth) (provider.Provider, error) {
	switch auth.Provider {
	case implementations.GithubProviderName:
		return implementations.NewGithub(auth, f.log), nil
	}

	return nil, fmt.Errorf("invalid provider name %q in auth %#v", auth.Provider, auth)
}

func (f BasicFactory) Build(auth *models.Auth) (provider.Provider, error) {
	p, err := f.buildImpl(auth)
	if err != nil {
		return nil, err
	}

	if err = f.hooksInjector.RunAfterProviderCreate(p); err != nil {
		return nil, errors.Wrap(err, "failed to run hooks after provider creation")
	}

	return implementations.NewStableProvider(p, time.Second*30, 3), nil
}

func (f BasicFactory) BuildForUser(db *gorm.DB, userID uint) (provider.Provider, error) {
	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(userID).One(&auth); err != nil {
		return nil, errors.Wrapf(err, "failed to get auth for user id %d", userID)
	}

	return f.Build(&auth)
}
