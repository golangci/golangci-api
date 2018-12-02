package organization

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

var ErrNotOrgAdmin = apierrors.NewNotAcceptableError("NOT_ORG_ADMIN").
	WithMessage("Only organization admins can view organization settings")

type AccessChecker struct {
	pf    providers.Factory
	cache cache.Cache
	cfg   config.Config
}

func NewAccessChecker(pf providers.Factory, cache cache.Cache, cfg config.Config) *AccessChecker {
	return &AccessChecker{
		pf:    pf,
		cache: cache,
		cfg:   cfg,
	}
}

func (ac AccessChecker) Check(rc *request.AuthorizedContext, org *models.Org) error {
	if org.ProviderPersonalUserID != 0 {
		if rc.Auth.ProviderUserID != uint64(org.ProviderPersonalUserID) {
			return errors.New("this is a personal org and this user doesn't own it")
		}

		return nil
	}

	provider, err := ac.pf.Build(rc.Auth)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	if provider.Name() != org.Provider {
		return errors.Wrapf(err, "auth provider %s != request org provider %s", provider.Name(), org.Provider)
	}

	providerOrg, fromCache, err := ac.fetchProviderOrgCached(rc, true, provider, org.Name)
	if err != nil {
		return errors.Wrap(err, "failed to fetch org from cached provider")
	}

	if !providerOrg.IsAdmin && fromCache { // user may have become an admin recently, refetch
		rc.Log.Infof("User isn't an admin in the result from cache, refetch it from the provider without cache")

		providerOrg, _, err = ac.fetchProviderOrgCached(rc, false, provider, org.Name)
		if err != nil {
			return errors.Wrap(err, "failed to fetch org from not cached provider")
		}
	}

	if !providerOrg.IsAdmin {
		return ErrNotOrgAdmin
	}

	rc.Log.Infof("User has access to the organization")
	return nil
}

func (ac AccessChecker) fetchProviderOrgCached(rc *request.AuthorizedContext, useCache bool,
	p provider.Provider, orgName string) (*provider.Org, bool, error) {

	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&org_name=%s&v=1", p.Name(), rc.Auth.UserID, orgName)

	var org *provider.Org
	if useCache {
		if err := ac.cache.Get(key, &org); err != nil {
			rc.Log.Warnf("Can't fetch org from cache by key %s: %s", key, err)
			providerOrg, fetchErr := ac.fetchProviderOrgFromProvider(rc, p, orgName)
			return providerOrg, false, fetchErr
		}

		if org != nil {
			rc.Log.Infof("Returning org(%d) from cache", org.ID)
			return org, true, nil
		}

		rc.Log.Infof("No org in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup org in cache, refreshing org from provider...")
	}

	var err error
	org, err = ac.fetchProviderOrgFromProvider(rc, p, orgName)
	if err != nil {
		return nil, false, err
	}

	cacheTTL := ac.cfg.GetDuration("ORG_CACHE_TTL", time.Hour*24*7)
	if err = ac.cache.Set(key, cacheTTL, org); err != nil {
		rc.Log.Warnf("Can't save org to cache by key %s: %s", key, err)
	}

	return org, false, nil
}

func (ac AccessChecker) fetchProviderOrgFromProvider(rc *request.AuthorizedContext, p provider.Provider, orgName string) (*provider.Org, error) {
	org, err := p.GetOrgByName(rc.Ctx, orgName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from provider by name %s", orgName)
	}

	return org, nil
}
