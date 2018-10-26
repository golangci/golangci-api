package provider

type Org struct {
	ID      int
	Name    string
	IsAdmin bool
}

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

type ListOrgsConfig struct {
	// MembershipState Indicates the state of the memberships to return.
	// Can be either active or pending.
	// If not specified, the API returns both active and pending memberships.
	MembershipState string
	MaxPages        int
}

type CommitStatus struct {
	Description string
	State       string
	Context     string
	TargetURL   string
}
