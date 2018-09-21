package models

import (
	"fmt"

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

func (a Auth) GoString() string {
	return fmt.Sprintf("{ID: %d, UserID: %d, Login: %s, Provider: %s}",
		a.ID, a.UserID, a.Login, a.Provider)
}
