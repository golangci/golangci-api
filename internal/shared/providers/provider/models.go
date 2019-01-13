package provider

import "strings"

type OrgMembership struct {
	ID      int
	Name    string
	IsAdmin bool
}

// Repo represents provider repository.
// On any incompatible change don't forget to bump cache version in fetchProviderReposCached
type Repo struct {
	ID            int
	FullName      string
	IsAdmin       bool
	IsPrivate     bool
	DefaultBranch string

	// The parent and source objects are present when the repository is a fork.
	// parent is the repository this repository was forked from,
	// source is the ultimate source for the network.
	Source *Repo

	StargazersCount int
	Language        string
	Organization    string
	OwnerID         int
}

func (r Repo) Name() string {
	return strings.Split(r.FullName, "/")[1]
}

func (r Repo) Owner() string {
	return strings.Split(r.FullName, "/")[0]
}

type Branch struct {
	CommitSHA string
}

type PullRequest struct {
	Head  *Branch
	State string // MERGED|CLOSED
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

type PullRequestAction string

const (
	Opened       PullRequestAction = "opened"
	Synchronized PullRequestAction = "synchronize"
)

type PullRequestEvent struct {
	Repo              *Repo
	Head              *Branch
	PullRequestNumber int
	Action            PullRequestAction
}

type CommitAuthor struct {
	Email string
}

type Commit struct {
	SHA       string
	Author    *CommitAuthor
	Committer *CommitAuthor
}

type RepoInvitation struct {
	ID                    int
	IsAlreadyCollaborator bool
}
