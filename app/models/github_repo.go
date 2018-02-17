package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in github_repo.go

// gen:qs
type GithubRepo struct {
	gorm.Model

	UserID       uint
	Name         string
	HookID       string
	GithubHookID int
}
