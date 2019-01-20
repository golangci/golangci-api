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
	of  *orgMembershipFetcher
	log logutil.Log
	cfg config.Config
}

func NewOrganization(pf providers.Factory, cache cache.Cache, cfg config.Config, log logutil.Log) *Organization {
	return &Organization{
		pf: pf,
		of: &orgMembershipFetcher{
			cache: cache,
			cfg:   cfg,
		},
		log: log,
		cfg: cfg,
	}
}

func checkCachedAdminMembership(om *provider.OrgMembership) error {
	if !om.IsAdmin {
		// user may have become an admin recently, refetch
		return errors.New("user isn't an admin in the organization")
	}

	return nil
}

func (op Organization) checkPersonalOrg(rc *request.AuthorizedContext, org *models.Org) error {
	if rc.Auth.ProviderUserID != uint64(org.ProviderPersonalUserID) {
		return fmt.Errorf("this is a personal org (%d) and this user (%d) doesn't own it",
			org.ProviderPersonalUserID, rc.Auth.ProviderUserID)
	}

	return nil
}

func (op Organization) buildProvider(rc *request.AuthorizedContext, org *models.Org) (provider.Provider, error) {
	p, err := op.pf.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	if p.Name() != org.Provider {
		return nil, errors.Wrapf(err, "auth provider %s != request org provider %s", p.Name(), org.Provider)
	}

	return p, nil
}

func (op Organization) checkIsAdmin(rc *request.AuthorizedContext, org *models.Org) error {
	if org.ProviderPersonalUserID != 0 {
		return op.checkPersonalOrg(rc, org)
	}

	p, err := op.buildProvider(rc, org)
	if err != nil {
		return err
	}

	orgMembership, err := op.of.fetch(rc, p, org.Name, checkCachedAdminMembership)
	if err != nil {
		causeErr := errors.Cause(err)
		if causeErr == provider.ErrNotFound {
			op.log.Warnf("Check org %s admin access: no read-only access to org: %s, return ErrNotOrgAdmin", org.Name, err)
			return ErrNotOrgAdmin
		}

		return err
	}

	if !orgMembership.IsAdmin {
		return ErrNotOrgAdmin
	}

	rc.Log.Infof("User has admin access to the organization %s", org.Name)
	return nil
}

func (op Organization) checkIsMember(rc *request.AuthorizedContext, org *models.Org) error {
	if org.ProviderPersonalUserID != 0 {
		return op.checkPersonalOrg(rc, org)
	}

	p, err := op.buildProvider(rc, org)
	if err != nil {
		return err
	}

	_, err = op.of.fetch(rc, p, org.Name, checkCachedAdminMembership)
	if err != nil {
		causeErr := errors.Cause(err)
		if causeErr == provider.ErrNotFound {
			op.log.Warnf("Check org %s membership: no read-only access to org: %s, return ErrNotOrgMember", org.Name, err)
			return ErrNotOrgMember
		}

		return err
	}

	rc.Log.Infof("User has membership in the organization %s", org.Name)
	return nil
}

func (op Organization) CheckCanModify(rc *request.AuthorizedContext, org *models.Org) error {
	if op.cfg.GetBool("PROVIDER_ORG_MEMBER_CAN_MODIFY_ORG", true) {
		rc.Log.Infof("Checking can modify org: check is member")
		return op.checkIsMember(rc, org)
	}

	rc.Log.Infof("Checking can modify org: check is admin")
	return op.checkIsAdmin(rc, org)
}
