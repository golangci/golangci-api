package returntypes

type RepoInfo struct {
	Name        string `json:"name"`
	IsActivated bool   `json:"isActivated,omitempty"`
	HookID      string `json:"hookId,omitempty"`
}

type AuthorizedUser struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatarUrl"`
	GithubLogin string `json:"githubLogin"`
}
