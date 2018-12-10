package organization

import (
	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/organization"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

type UpdatePayload struct {
	Settings *models.Settings `json:"settings"`
	Version  int              `json:"version"`
}

func (p UpdatePayload) FillLogContext(lctx logutil.Context) {
	lctx["version"] = p.Version
}

type Service interface {
	//url:/v1/orgs/{provider}/{name} method:PUT
	Update(rc *request.AuthorizedContext, reqOrg *request.Org, payload *UpdatePayload) (*models.Org, error)

	//url:/v1/orgs/{provider}/{name}
	Get(rc *request.AuthorizedContext, reqOrg *request.Org) (*models.Org, error)
}

type BasicService struct {
	ac *organization.AccessChecker
}

func NewBasicService(ac *organization.AccessChecker) *BasicService {
	return &BasicService{
		ac: ac,
	}
}

func (s BasicService) Update(rc *request.AuthorizedContext, reqOrg *request.Org, payload *UpdatePayload) (*models.Org, error) {
	qs := models.NewOrgQuerySet(rc.DB).NameEq(reqOrg.Name).ProviderEq(reqOrg.Provider)
	var org models.Org
	if err := qs.One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to to get org from db")
	}

	if org.Version != payload.Version {
		return nil, apierrors.NewRaceConditionError("organization settings were changed in parallel")
	}

	if err := s.ac.Check(rc, &org); err != nil {
		return nil, errors.Wrap(err, "check access to org")
	}

	if err := org.MarshalSettings(payload.Settings); err != nil {
		return nil, errors.Wrapf(err, "failed to set settings for %d", org.ID)
	}

	upd := qs.VersionEq(org.Version).GetUpdater().SetSettings(org.Settings).SetVersion(org.Version + 1)
	if err := upd.UpdateRequired(); err != nil {
		return nil, errors.Wrapf(err, "failed to commit settings change for %d", org.ID)
	}
	org.Version++

	return &org, nil
}

func (s *BasicService) Get(rc *request.AuthorizedContext, reqOrg *request.Org) (*models.Org, error) {
	var org models.Org
	if err := models.NewOrgQuerySet(rc.DB).NameEq(reqOrg.Name).ProviderEq(reqOrg.Provider).One(&org); err != nil {
		return nil, errors.Wrap(err, "failed to to get org from db")
	}

	if err := s.ac.Check(rc, &org); err != nil {
		return nil, errors.Wrap(err, "check access to org")
	}

	return &org, nil
}
