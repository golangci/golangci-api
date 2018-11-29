package returntypes

import (
	"time"

	"github.com/golangci/golangci-api/pkg/app/models"
)

type Error struct {
	Error string `json:"error,omitempty"`
}

type RepoInfo struct {
	ID           uint   `json:"id"`
	HookID       string `json:"hookId"` // needed only for tests
	Name         string `json:"name"`
	Organization string `json:"organization,omitempty"`
	IsAdmin      bool   `json:"isAdmin"`
	IsActivated  bool   `json:"isActivated,omitempty"`
	IsPrivate    bool   `json:"isPrivate,omitempty"`
	IsCreating   bool   `json:"isCreating,omitempty"`
	IsDeleting   bool   `json:"isDeleting,omitempty"`
	Language     string `json:"language,omitempty"`
}

type WrappedRepoInfo struct {
	Repo RepoInfo `json:"repo"`
}

type RepoListResponse struct {
	Repos                   []RepoInfo `json:"repos"`
	PrivateRepos            []RepoInfo `json:"privateRepos"`
	PrivateReposWereFetched bool       `json:"privateReposWereFetched"`
}

type AuthorizedUser struct {
	ID          uint      `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatarUrl"`
	GithubLogin string    `json:"githubLogin"`
	CreatedAt   time.Time `json:"createdAt"`
}

type CheckAuthResponse struct {
	User AuthorizedUser `json:"user"`
}

type SubInfo struct {
	ID         uint   `json:"id"`
	SeatsCount int    `json:"seatsCount"`
	Status     string `json:"status"`
}

//nolint:gocritic
func SubFromModel(sub models.OrgSub) *SubInfo {
	status := "active"
	if sub.IsCreating() {
		status = "creating"
	} else if sub.IsUpdating() {
		status = "updating"
	} else if sub.IsDeleting() || sub.CommitState == models.OrgSubCommitStateDeleteDone {
		status = "deleted"
	}
	return &SubInfo{sub.ID, sub.SeatsCount, status}
}

type OrgInfo struct {
	ID           uint     `json:"id"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	IsAdmin      bool     `json:"isAdmin"`
	Subscription *SubInfo `json:"subscription"`
}

func OrgFromModel(org models.Org, admin bool) *OrgInfo {
	return &OrgInfo{org.ID, org.Name, org.DisplayName, admin, nil}
}

type IDResponse struct {
	ID int `json:"id"`
}
