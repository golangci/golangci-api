package repos

import (
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/internal/errors"
	"github.com/golangci/golangci-api/app/internal/github"
	"github.com/golangci/golangci-api/app/internal/repos"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
)

func fetchGithubRepos(ctx *context.C, client *gh.Client, maxPageNumber int) ([]*gh.Repository, error) {
	// list all repositories for the authenticated user
	vis := "public"
	if repos.ArePrivateReposEnabledForUser(ctx) {
		vis = "all"
	}

	opts := gh.RepositoryListOptions{
		Visibility: vis,
		Sort:       "pushed",
	}
	var allRepos []*gh.Repository
	for {
		pageRepos, resp, err := client.Repositories.List(ctx.Ctx, "", &opts)
		if err != nil {
			return nil, fmt.Errorf("can't get repos list: %s", err)
		}

		allRepos = append(allRepos, pageRepos...)

		if resp.NextPage == 0 { // it's a last page
			break
		}

		if opts.Page == maxPageNumber { // TODO: fetch all, now we limit it to maxPageNumber pages
			errors.Warnf(ctx, "Limited repo list to %d entries", len(allRepos))
			break
		}

		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func getActivatedUserRepos(ctx *context.C) (map[string]*models.GithubRepo, error) {
	ga, err := user.GetGithubAuth(ctx)
	if err != nil {
		return nil, herrors.New(err, "can't get current github auth")
	}

	var repos []models.GithubRepo
	err = models.NewGithubRepoQuerySet(db.Get(ctx)).UserIDEq(ga.UserID).All(&repos)
	if err != nil {
		return nil, fmt.Errorf("can't select activated repos from db: %s", err)
	}

	ret := map[string]*models.GithubRepo{}
	for _, r := range repos {
		ret[strings.ToLower(r.Name)] = &r
	}

	ctx.L.Infof("user %d repos: %v, map: %v", ga.UserID, repos, ret)

	return ret, nil
}

func getReposList(ctx context.C) error {
	client, err := github.GetClient(&ctx)
	if err != nil {
		return herrors.New(err, "can't get github client")
	}

	repos, err := fetchGithubRepos(&ctx, client, 3)
	if err != nil {
		return herrors.New(err, "can't fetch repos from github")
	}

	activatedRepos, err := getActivatedUserRepos(&ctx)
	if err != nil {
		return herrors.New(err, "can't get activated repos for user")
	}

	ret := []returntypes.RepoInfo{}
	for _, r := range repos {
		ar := activatedRepos[strings.ToLower(r.GetFullName())]
		hookID := ""
		if ar != nil {
			hookID = ar.HookID
		}
		ret = append(ret, returntypes.RepoInfo{
			Name:        r.GetFullName(),
			IsActivated: ar != nil,
			HookID:      hookID,
		})
	}

	ctx.ReturnJSON(map[string]interface{}{
		"repos": ret,
	})
	return nil
}

func init() {
	handlers.Register("/v1/repos", getReposList)
}
