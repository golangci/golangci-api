package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in auth.go

// gen:qs
type Auth struct {
	gorm.Model

	AccessToken        string
	PrivateAccessToken string

	RawData []byte
	UserID  uint

	Provider       string
	ProviderUserID uint64

	Login string
}
