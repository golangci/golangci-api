package returntypes

import "time"

type RepoInfo struct {
	Name        string `json:"name"`
	IsAdmin     bool   `json:"isAdmin"`
	IsActivated bool   `json:"isActivated,omitempty"`
	IsPrivate   bool   `json:"isPrivate,omitempty"`
	HookID      string `json:"hookId,omitempty"`
}

type AuthorizedUser struct {
	ID          uint      `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatarUrl"`
	GithubLogin string    `json:"githubLogin"`
	CreatedAt   time.Time `json:"createdAt"`
}
