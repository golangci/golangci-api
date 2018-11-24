package provider

type Repo struct {
	ID            int
	Name          string
	IsAdmin       bool
	IsPrivate     bool
	DefaultBranch string

	// The parent and source objects are present when the repository is a fork.
	// parent is the repository this repository was forked from,
	// source is the ultimate source for the network.
	Source *Repo

	StargazersCount int
}

type Branch struct {
	HeadCommitSHA string
}

type PullRequest struct {
	HeadCommitSHA string
	State         string // MERGED|CLOSED
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
