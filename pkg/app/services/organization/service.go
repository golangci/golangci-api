package organization

import (
	"encoding/json"
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
	} `json:"seats"`
}

func settingsFromOrg(org models.Org) (*Settings, error) {
	var settings *Settings
	return settings, errors.Wrapf(json.Unmarshal(org.Settings, settings), "failed to unmarshal settings for org(%d)", org.ID)
}

type SettingsWrapped struct {
	Settings Settings `json:"settings"`
}

type OrgSettingsBody struct {
	SettingsWrapped
	request.OrgID
}

func (r OrgSettingsBody) FillLogContext(lctx logutil.Context) {
	r.OrgID.FillLogContext(lctx)
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
	Update(rc *request.AuthorizedContext, reqOrg *OrgSettingsBody) error

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

func (s *basicService) Update(rc *request.AuthorizedContext, reqOrg *OrgSettingsBody) error {
	return errors.New("not implemented")
}

func (s *basicService) Get(rc *request.AuthorizedContext, reqOrg *request.OrgID) (*SettingsWrapped, error) {
	return nil, errors.New("not implemented")
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
	if rc.Auth.PrivateAccessToken != "" {
		key += "&private=true"
	}

	var orgs []provider.Org
	if useCache {
		if err := s.Cache.Get(key, &orgs); err != nil {
			rc.Log.Warnf("Can't fetch repos from cache by key %s: %s", key, err)
			return s.fetchProviderOrgsFromProvider(rc, p, maxPages)
		}

		if len(orgs) != 0 {
			rc.Log.Infof("Returning %d repos from cache", len(orgs))
			return orgs, nil
		}

		rc.Log.Infof("No repos in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup repos in cache, refreshing repos from provider...")
	}

	var err error
	orgs, err = s.fetchProviderOrgsFromProvider(rc, p, maxPages)
	if err != nil {
		return nil, err
	}

	cacheTTL := s.Cfg.GetDuration("ORGS_CACHE_TTL", time.Hour*24*7)
	if err = s.Cache.Set(key, cacheTTL, orgs); err != nil {
		rc.Log.Warnf("Can't save %d repos to cache by key %s: %s", len(orgs), key, err)
	}

	return orgs, nil
}

func (s basicService) fetchProviderOrgsFromProvider(rc *request.AuthorizedContext, p provider.Provider, maxPages int) ([]provider.Org, error) {
	// repos, err := p
	orgs, err := p.ListOrgs(rc.Ctx, &provider.ListOrgsConfig{
		MaxPages: maxPages,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch orgs from provider %s", p.Name())
	}

	return orgs, nil
}
