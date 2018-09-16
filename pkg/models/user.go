package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in user.go

// gen:qs
type User struct {
	gorm.Model

	Email string

	Name      string
	AvatarURL string
}
