package fetchers

type Repo struct {
	CloneURL  string
	Ref       string
	CommitSHA string
	FullPath  string
}
