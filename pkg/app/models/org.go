package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in org.go

// gen:qs
type Org struct {
	gorm.Model

	Name        string
	DisplayName string

	Provider               string
	ProviderID             int
	ProviderPersonalUserID int

	Settings []byte
}

func (o *Org) IsFake() bool {
	return o.ProviderPersonalUserID != 0
}
