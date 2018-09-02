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
	Name         string // lower-cased DisplayName to avoid case-sensitivity bugs
	DisplayName  string // original name of repo from github: original register is saved
	HookID       string
	GithubHookID int
	GithubID     int // github repo id: use it (not name) as repo identifier because of repo renaming
}

func (r *GithubRepo) Owner() string {
	return strings.ToLower(strings.Split(r.Name, "/")[0])
}

func (r *GithubRepo) Repo() string {
	return strings.ToLower(strings.Split(r.Name, "/")[1])
}
