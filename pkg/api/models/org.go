package models

import (
	"encoding/json"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

//go:generate goqueryset -in org.go

// gen:qs
type Org struct {
	gorm.Model `json:"-"`

	Name        string `json:"-"`
	DisplayName string `json:"name"`

	Provider               string `json:"provider"`
	ProviderID             int    `json:"-"`
	ProviderPersonalUserID int    `json:"-"`

	Settings json.RawMessage `json:"settings"`
	Version  int             `json:"version"`
}

func (o *Org) IsFake() bool {
	return o.ProviderPersonalUserID != 0
}

func (o *Org) UnmarshalSettings() (*OrgSettings, error) {
	var s OrgSettings
	if err := json.Unmarshal(o.Settings, &s); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal settings for org(%d)", o.ID)
	}

	return &s, nil
}

func (o *Org) MarshalSettings(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal %#v as settings", v)
	}
	o.Settings = data
	return nil
}

type OrgSeat struct {
	Email string `json:"email"`
}

type OrgSettings struct {
	Seats []OrgSeat `json:"seats,omitempty"`
}

func (u OrgUpdater) UpdateRequired() error {
	n, err := u.UpdateNum()
	if err != nil {
		return err
	}

	if n == 0 {
		return apierrors.NewRaceConditionError("data was changed in parallel request")
	}

	return nil
}

func (qs OrgQuerySet) ForProviderRepo(providerName, orgName string, providerOwnerID int) OrgQuerySet {
	qs = qs.ProviderEq(providerName)
	if orgName == "" {
		qs = qs.ProviderPersonalUserIDEq(providerOwnerID)
	} else {
		qs = qs.ProviderIDEq(providerOwnerID)
	}

	return qs
}
