package returntypes

import "time"

type RepoInfo struct {
	ID          uint   `json:"id"`
	HookID      string `json:"hookId"` // needed only for tests
	Name        string `json:"name"`
	IsAdmin     bool   `json:"isAdmin"`
	IsActivated bool   `json:"isActivated,omitempty"`
	IsPrivate   bool   `json:"isPrivate,omitempty"`
	IsCreating  bool   `json:"isCreating,omitempty"`
	IsDeleting  bool   `json:"isDeleting,omitempty"`
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
