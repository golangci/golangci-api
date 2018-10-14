package provider

type Repo struct {
	ID            int
	Name          string
	IsAdmin       bool
	IsPrivate     bool
	DefaultBranch string
}

type Branch struct {
	HeadCommitSHA string
}

type PullRequest struct {
	HeadCommitSHA string
}

type HookConfig struct {
	Name        string
	Events      []string
	URL         string
	ContentType string
}

type Hook struct {
	ID int
	HookConfig
}

type ListReposConfig struct {
	Visibility string // public|all
	Sort       string
	MaxPages   int
}

type CommitStatus struct {
	Description string
	State       string
	Context     string
	TargetURL   string
}
