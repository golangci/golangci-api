package providers

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers/implementations"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Factory interface {
	Build(auth *models.Auth) (provider.Provider, error)
	BuildForUser(db *gorm.DB, userID uint) (provider.Provider, error)
}

type BasicFactory struct {
	log logutil.Log
}

func NewBasicFactory(log logutil.Log) *BasicFactory {
	return &BasicFactory{
		log: log,
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

	return implementations.NewStableProvider(p, time.Second*30, 3), nil
}

func (f BasicFactory) BuildForUser(db *gorm.DB, userID uint) (provider.Provider, error) {
	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(userID).One(&auth); err != nil {
		return nil, errors.Wrapf(err, "failed to get auth for user id %d", userID)
	}

	return f.Build(&auth)
}
