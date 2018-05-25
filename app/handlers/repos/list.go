package repos

import (
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golangci-api/app/internal/cache"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/internal/errors"
	"github.com/golangci/golangci-api/app/internal/github"
	"github.com/golangci/golangci-api/app/internal/repos"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	gh "github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

type ShortRepoInfo struct {
	FullName  string
	IsAdmin   bool `json:",omitempty"`
	IsPrivate bool `json:",omitempty"`
}

func fetchGithubReposCached(ctx *context.C, client *gh.Client, maxPageNumber int) ([]ShortRepoInfo, error) {
	userID, err := user.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("repos/github/fetch?user_id=%d&maxPage=%d&v=2", userID, maxPageNumber)
	if repos.ArePrivateReposEnabledForUser(ctx) {
		key += "&private=true"
	}

	c := cache.Get()

	var repos []ShortRepoInfo
	if ctx.R.URL.Query().Get("refresh") != "1" { // Don't refresh from github
		if err = c.Get(key, &repos); err != nil {
			errors.Warnf(ctx, "Can't fetch repos from cache by key %s: %s", key, err)
			return fetchGithubReposFromGithub(ctx, client, maxPageNumber)
		}

		if repos != nil {
			logrus.Infof("Returning %d repos from cache", len(repos))
			return repos, nil
		}

		logrus.Infof("No repos in cache, fetching them from github...")
	} else {
		logrus.Infof("Don't lookup repos in cache, refreshing repos from github...")
	}

	repos, err = fetchGithubReposFromGithub(ctx, client, maxPageNumber)
	if err != nil {
		return nil, err
	}

	if err = c.Set(key, time.Hour*24*7, repos); err != nil {
		errors.Warnf(ctx, "Can't save %d repos to cache by key %s: %s", len(repos), key, err)
	}

	return repos, nil
}

func fetchGithubReposFromGithub(ctx *context.C, client *gh.Client, maxPageNumber int) ([]ShortRepoInfo, error) {
	// list all repositories for the authenticated user
	vis := "public"
	if repos.ArePrivateReposEnabledForUser(ctx) {
		vis = "all"
	}

	opts := gh.RepositoryListOptions{
		Visibility: vis,
		Sort:       "pushed",
		ListOptions: gh.ListOptions{
			PerPage: 100, // 100 is a max allowed value
		},
	}
	var allRepos []ShortRepoInfo
	for {
		pageRepos, resp, err := client.Repositories.List(ctx.Ctx, "", &opts)
		if err != nil {
			return nil, fmt.Errorf("can't get repos list: %s", err)
		}

		for _, r := range pageRepos {
			allRepos = append(allRepos, ShortRepoInfo{
				FullName:  r.GetFullName(),
				IsAdmin:   r.GetPermissions()["admin"],
				IsPrivate: r.GetPrivate(),
			})
		}

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
	client, needPrivateRepos, err := github.GetClient(&ctx)
	if err != nil {
		return herrors.New(err, "can't get github client")
	}

	repos, err := fetchGithubReposCached(&ctx, client, 20)
	if err != nil {
		return herrors.New(err, "can't fetch repos from github")
	}

	activatedRepos, err := getActivatedUserRepos(&ctx)
	if err != nil {
		return herrors.New(err, "can't get activated repos for user")
	}

	retRepos := []returntypes.RepoInfo{}
	retPrivateRepos := []returntypes.RepoInfo{}
	for _, r := range repos {
		ar := activatedRepos[strings.ToLower(r.FullName)]
		hookID := ""
		if ar != nil {
			hookID = ar.HookID
		}
		retRepo := returntypes.RepoInfo{
			Name:        r.FullName,
			IsAdmin:     r.IsAdmin,
			IsActivated: ar != nil,
			IsPrivate:   r.IsPrivate,
			HookID:      hookID,
		}
		if retRepo.IsPrivate {
			retPrivateRepos = append(retPrivateRepos, retRepo)
		} else {
			retRepos = append(retRepos, retRepo)
		}
	}

	ctx.ReturnJSON(map[string]interface{}{
		"repos":                   retRepos,
		"privateRepos":            retPrivateRepos,
		"privateReposWereFetched": needPrivateRepos,
	})
	return nil
}

func init() {
	handlers.Register("/v1/repos", getReposList)
}
