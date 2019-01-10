package policy

import (
	"fmt"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

type Organization struct {
	pf  providers.Factory
	of  *orgFetcher
	log logutil.Log
}

func NewOrganization(pf providers.Factory, cache cache.Cache, cfg config.Config, log logutil.Log) *Organization {
	return &Organization{
		pf: pf,
		of: &orgFetcher{
			cache: cache,
			cfg:   cfg,
		},
		log: log,
	}
}

func (op Organization) CheckAdminAccess(rc *request.AuthorizedContext, org *models.Org) error {
	if org.ProviderPersonalUserID != 0 {
		if rc.Auth.ProviderUserID != uint64(org.ProviderPersonalUserID) {
			return fmt.Errorf("this is a personal org (%d) and this user (%d) doesn't own it",
				org.ProviderPersonalUserID, rc.Auth.ProviderUserID)
		}

		return nil
	}

	p, err := op.pf.Build(rc.Auth)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	if p.Name() != org.Provider {
		return errors.Wrapf(err, "auth provider %s != request org provider %s", p.Name(), org.Provider)
	}

	providerOrg, err := op.of.fetch(rc, p, org.Name)
	if err != nil {
		causeErr := errors.Cause(err)
		if causeErr == provider.ErrNotFound {
			op.log.Warnf("Check org %s admin access: no read-only access to org: %s, return ErrNotOrgAdmin", org.Name, err)
			return ErrNotOrgAdmin
		}

		return err
	}

	if !providerOrg.IsAdmin {
		return ErrNotOrgAdmin
	}

	rc.Log.Infof("User has admin access to the organization %s", org.Name)
	return nil
}
