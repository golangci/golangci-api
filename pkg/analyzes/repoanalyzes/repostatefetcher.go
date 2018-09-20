package repoanalyzes

import (
	"context"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type GithubRepoState struct {
	DefaultBranch string
	HeadCommitSHA string
}

type GithubRepoStateFetcher struct {
	db *gorm.DB
}

func NewGithubRepoStateFetcher(db *gorm.DB) *GithubRepoStateFetcher {
	return &GithubRepoStateFetcher{
		db: db,
	}
}

func (f GithubRepoStateFetcher) Fetch(ctx context.Context, repo *models.Repo) (*GithubRepoState, error) {
	gc, err := github.GetClientForUserV2(ctx, f.db, repo.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "can't get github client")
	}

	gr, _, err := gc.Repositories.Get(ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo %s from github", repo.Name)
	}

	defaultBranch := gr.GetDefaultBranch()
	grb, _, err := gc.Repositories.GetBranch(ctx, repo.Owner(), repo.Repo(), defaultBranch)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get github branch %s info", defaultBranch)
	}

	return &GithubRepoState{
		DefaultBranch: defaultBranch,
		HeadCommitSHA: grb.GetCommit().GetSHA(),
	}, nil
}
