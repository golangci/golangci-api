package models

import (
	"strings"

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

func (r *GithubRepo) Owner() string {
	return strings.ToLower(strings.Split(r.Name, "/")[0])
}

func (r *GithubRepo) Repo() string {
	return strings.ToLower(strings.Split(r.Name, "/")[1])
}
