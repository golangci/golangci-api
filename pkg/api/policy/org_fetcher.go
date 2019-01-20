package policy

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

type orgMembershipFetcher struct {
	cache cache.Cache
	cfg   config.Config
}

type cachedMembershipChecker func(om *provider.OrgMembership) error

func (of orgMembershipFetcher) fetch(rc *request.AuthorizedContext, p provider.Provider, orgName string, checker cachedMembershipChecker) (*provider.OrgMembership, error) {
	orgMembership, fromCache, err := of.fetchCached(rc, true, p, orgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch org from cached provider")
	}

	if !fromCache || checker == nil {
		return orgMembership, nil
	}

	cachedOrgMembership := orgMembership

	if err = checker(orgMembership); err != nil {
		rc.Log.Infof("Refetching org membership it from the provider without cache: %s", err)

		orgMembership, _, err = of.fetchCached(rc, false, p, orgName)
		if err != nil {
			rc.Log.Warnf("Failed to fetch org from not cached provider, fallback to the cached data")
			return cachedOrgMembership, nil
		}
	}

	return orgMembership, nil
}

func (of orgMembershipFetcher) fetchCached(rc *request.AuthorizedContext, useCache bool,
	p provider.Provider, orgName string) (*provider.OrgMembership, bool, error) {

	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&org_name=%s&v=1", p.Name(), rc.Auth.UserID, orgName)

	var org *provider.OrgMembership
	if useCache {
		if err := of.cache.Get(key, &org); err != nil {
			rc.Log.Warnf("Can't fetch org from cache by key %s: %s", key, err)
			providerOrg, fetchErr := of.fetchFromProvider(rc, p, orgName)
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
	org, err = of.fetchFromProvider(rc, p, orgName)
	if err != nil {
		return nil, false, err
	}

	cacheTTL := of.cfg.GetDuration("ORG_CACHE_TTL", time.Hour*24*7)
	if err = of.cache.Set(key, cacheTTL, org); err != nil {
		rc.Log.Warnf("Can't save org to cache by key %s: %s", key, err)
	}

	return org, false, nil
}

func (of orgMembershipFetcher) fetchFromProvider(rc *request.AuthorizedContext, p provider.Provider, orgName string) (*provider.OrgMembership, error) {
	org, err := p.GetOrgMembershipByName(rc.Ctx, orgName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from provider by name %s", orgName)
	}

	return org, nil
}
