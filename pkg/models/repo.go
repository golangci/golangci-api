package models

import (
	"strings"

	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in repo.go

// gen:qs
type Repo struct {
	gorm.Model

	// TODO: remove it and move repo connections to another table,
	// take organizations into account
	UserID uint // user who the last time connected this repo

	Name        string // lower-cased DisplayName to avoid case-sensitivity bugs
	DisplayName string // original name of repo from github: original register is saved

	HookID string

	// GitHub specific
	GithubHookID int
	GithubID     int // github repo id: use it (not name) as repo identifier because of repo renaming
}

func (r *Repo) Owner() string {
	return strings.ToLower(strings.Split(r.Name, "/")[0])
}

func (r *Repo) Repo() string {
	return strings.ToLower(strings.Split(r.Name, "/")[1])
}

func (r *Repo) String() string {
	return r.Name
}
