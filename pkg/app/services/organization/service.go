package organization // nolint:dupl

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
	"github.com/golangci/golangci-api/pkg/app/returntypes"
	"github.com/golangci/golangci-api/pkg/cache"
	"github.com/golangci/golangci-api/pkg/endpoint/request"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Settings struct {
	Seats []struct {
		Email string `json:"email"`
	} `json:"seats,omitempty"`
}

type SettingsWrapped struct {
	Settings *Settings `json:"settings"`
}

func (r SettingsWrapped) FillLogContext(lctx logutil.Context) {

}

type OrgList struct {
	Organizations []*returntypes.OrgInfo `json:"organizations"`
}

type OrgListRequest struct {
	Refresh bool `request:",urlParam,optional"`
}

func (r OrgListRequest) FillLogContext(lctx logutil.Context) {
	lctx["refresh"] = r.Refresh
}

type Service interface {
	//url:/v1/orgs/{org_id} method:PUT
	Update(rc *request.AuthorizedContext, context *request.OrgID, settings *SettingsWrapped) error

	//url:/v1/orgs/{org_id}
	Get(rc *request.AuthorizedContext, reqOrg *request.OrgID) (*SettingsWrapped, error)

	//url:/v1/orgs
	List(rc *request.AuthorizedContext, reqOrg *OrgListRequest) (*OrgList, error)
}

func Configure(providerFactory providers.Factory, cache cache.Cache, cfg config.Config) Service {
	return &basicService{providerFactory, cache, cfg}
}

type basicService struct {
	ProviderFactory providers.Factory
	Cache           cache.Cache
	Cfg             config.Config
}

func (s *basicService) Update(rc *request.AuthorizedContext, context *request.OrgID, settings *SettingsWrapped) error {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).IDEq(context.OrgID).One(&org); err != nil {
		return errors.Wrapf(err, "failed to get org from db with id %d", context.OrgID)
	}
	if org.ProviderPersonalUserID != 0 {
		if rc.Auth.ProviderUserID != uint64(org.ProviderPersonalUserID) {
			return errors.New("this is a personal org and this user doesn't own it")
		}
	} else {
		provider, err := s.ProviderFactory.Build(rc.Auth)
		if err != nil {
			return errors.Wrap(err, "failed to build provider")
		}

		if provider.Name() != org.Provider {
			return errors.Wrapf(err, "auth provider %s != request org provider %s", provider.Name(), org.Provider)
		}

		org, err := s.fetchProviderOrgCached(rc, false, provider, org.ProviderID)

		if err != nil {
			return errors.Wrap(err, "failed to fetch org from provider")
		}

		if !org.IsAdmin {
			return errors.New("no admin permission on org")
		}
	}

	if err := org.MarshalSettings(settings.Settings); err != nil {
		return errors.Wrapf(err, "failed to set settings for %d", org.ID)
	}

	if err := org.Update(rc.DB, models.OrgDBSchema.Settings); err != nil {
		return errors.Wrapf(err, "failed to commit settings change for %d", org.ID)
	}

	return nil
}

func (s *basicService) Get(rc *request.AuthorizedContext, reqOrg *request.OrgID) (*SettingsWrapped, error) {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB.Unscoped()).IDEq(reqOrg.OrgID).One(&org); err != nil {
		return nil, errors.Wrapf(err, "failed to to get org from db with id %d", reqOrg.OrgID)
	}

	var settings Settings
	if err := org.UnmarshalSettings(&settings); err != nil {
		return nil, err
	}

	return &SettingsWrapped{&settings}, nil
}

func (s *basicService) List(rc *request.AuthorizedContext, req *OrgListRequest) (*OrgList, error) {
	provider, err := s.ProviderFactory.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	providerOrgs, err := s.fetchProviderOrgsCached(rc, !req.Refresh, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch orgs from provider")
	}

	retOrgs := []*returntypes.OrgInfo{}

	org, err := s.fetchOrg(rc, provider.Name(), int(rc.Auth.ProviderUserID), true, true)
	if err != nil {
		// TODO(all): Spec suggests all errors should be wrapped but this error is already wrapped upstream
		// So not sure where to go from here...
		return nil, err
	} else if org != nil {
		org.Subscription, err = s.fetchOrgSub(rc, org.ID)
		if err != nil {
			return nil, err
		}
		retOrgs = append(retOrgs, org)
	}

	for _, po := range providerOrgs {
		org, err := s.fetchOrg(rc, provider.Name(), po.ID, false, po.IsAdmin)
		if err != nil {
			// TODO(all): Spec suggests all errors should be wrapped but this error is already wrapped upstream
			// So not sure where to go from here...
			return nil, err
		} else if org == nil {
			// Doesn't have an org... carry on for now.
			continue
		}
		org.Subscription, err = s.fetchOrgSub(rc, org.ID)
		if err != nil {
			return nil, err
		}
		retOrgs = append(retOrgs, org)
	}

	return &OrgList{retOrgs}, nil
}

func (s basicService) fetchOrgSub(rc *request.AuthorizedContext, id uint) (*returntypes.SubInfo, error) {
	var orgSub models.OrgSub
	err := models.NewOrgSubQuerySet(rc.DB).OrgIDEq(id).One(&orgSub)
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org sub from db for %d", id)
	}
	return returntypes.SubFromModel(orgSub), nil
}

func (s basicService) fetchOrg(rc *request.AuthorizedContext, provider string, id int, personalOrg bool, admin bool) (*returntypes.OrgInfo, error) {
	var org models.Org
	var err error
	baseQuery := models.NewOrgQuerySet(rc.DB).ProviderEq(provider)
	if personalOrg {
		err = baseQuery.ProviderPersonalUserIDEq(id).One(&org)
	} else {
		err = baseQuery.ProviderIDEq(id).One(&org)
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from db for {id: %d, personal: %v}", id, personalOrg)
	}
	return returntypes.OrgFromModel(org, admin), nil
}

func (s basicService) fetchProviderOrgsCached(rc *request.AuthorizedContext, useCache bool, p provider.Provider) ([]provider.Org, error) {
	const maxPages = 20
	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&maxPage=%d&v=1", p.Name(), rc.Auth.UserID, maxPages)

	var orgs []provider.Org
	if useCache {
		if err := s.Cache.Get(key, &orgs); err != nil {
			rc.Log.Warnf("Can't fetch orgs from cache by key %s: %s", key, err)
			return s.fetchProviderOrgsFromProvider(rc, p, maxPages)
		}

		if len(orgs) != 0 {
			rc.Log.Infof("Returning %d orgs from cache", len(orgs))
			return orgs, nil
		}

		rc.Log.Infof("No orgs in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup orgs in cache, refreshing orgs from provider...")
	}

	var err error
	orgs, err = s.fetchProviderOrgsFromProvider(rc, p, maxPages)
	if err != nil {
		return nil, err
	}

	cacheTTL := s.Cfg.GetDuration("ORGS_CACHE_TTL", time.Hour*24*7)
	if err = s.Cache.Set(key, cacheTTL, orgs); err != nil {
		rc.Log.Warnf("Can't save %d orgs to cache by key %s: %s", len(orgs), key, err)
	}

	return orgs, nil
}

func (s basicService) fetchProviderOrgsFromProvider(rc *request.AuthorizedContext, p provider.Provider, maxPages int) ([]provider.Org, error) {
	orgs, err := p.ListOrgs(rc.Ctx, &provider.ListOrgsConfig{
		MaxPages: maxPages,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch orgs from provider %s", p.Name())
	}

	return orgs, nil
}

func (s basicService) fetchProviderOrgCached(rc *request.AuthorizedContext, useCache bool, p provider.Provider, oid int) (*provider.Org, error) {
	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&org_id=%d&v=1", p.Name(), rc.Auth.UserID, oid)

	var org *provider.Org
	if useCache {
		if err := s.Cache.Get(key, &org); err != nil {
			rc.Log.Warnf("Can't fetch org from cache by key %s: %s", key, err)
			return s.fetchProviderOrgFromProvider(rc, p, oid)
		}

		if org != nil {
			rc.Log.Infof("Returning org(%d) from cache", org.ID)
			return org, nil
		}

		rc.Log.Infof("No org in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup org in cache, refreshing org from provider...")
	}

	var err error
	org, err = s.fetchProviderOrgFromProvider(rc, p, oid)
	if err != nil {
		return nil, err
	}

	cacheTTL := s.Cfg.GetDuration("ORG_CACHE_TTL", time.Hour*24*7)
	if err = s.Cache.Set(key, cacheTTL, org); err != nil {
		rc.Log.Warnf("Can't save org to cache by key %s: %s", key, err)
	}

	return org, nil
}

func (s basicService) fetchProviderOrgFromProvider(rc *request.AuthorizedContext, p provider.Provider, oid int) (*provider.Org, error) {
	org, err := p.GetOrgByID(rc.Ctx, oid)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from provider by id %d", oid)
	}

	return org, nil
}
