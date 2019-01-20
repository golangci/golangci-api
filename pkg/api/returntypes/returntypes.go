package returntypes

import (
	"time"
)

type Error struct {
	Error string `json:"error,omitempty"`
}

type RepoInfo struct {
	ID               uint   `json:"id"`
	HookID           string `json:"hookId"` // needed only for tests
	Name             string `json:"name"`
	Organization     string `json:"organization,omitempty"`
	IsAdmin          bool   `json:"isAdmin"`
	IsActivated      bool   `json:"isActivated,omitempty"`
	IsPrivate        bool   `json:"isPrivate,omitempty"`
	IsCreating       bool   `json:"isCreating,omitempty"`
	IsDeleting       bool   `json:"isDeleting,omitempty"`
	Language         string `json:"language,omitempty"`
	CreateFailReason string `json:"createFailReason,omitempty"`
}

type WrappedRepoInfo struct {
	Repo RepoInfo `json:"repo"`
}

type OrgInfo struct {
	Provider              string `json:"provider"`
	Name                  string `json:"name"`
	HasActiveSubscription bool   `json:"hasActiveSubscription"`
	CanModify             bool   `json:"canModify"`
	CantModifyReason      string `json:"cantModifyReason"`
}

type RepoListResponse struct {
	Repos                   []RepoInfo         `json:"repos"`
	PrivateRepos            []RepoInfo         `json:"privateRepos"`
	PrivateReposWereFetched bool               `json:"privateReposWereFetched"`
	Organizations           map[string]OrgInfo `json:"organizations"`
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
	SeatsCount   int    `json:"seatsCount"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	PricePerSeat string `json:"pricePerSeat"`
	CancelURL    string `json:"cancelUrl"`

	TrialAllowanceInDays int    `json:"trialAllowanceInDays"`
	PaddleTrialDaysAuth  string `json:"paddleTrialDaysAuth"`
}

type IDResponse struct {
	ID int `json:"id"`
}
