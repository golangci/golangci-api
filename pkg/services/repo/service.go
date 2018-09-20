package repo

import (
	"fmt"
	"os"
	"strings"

	"github.com/golangci/golangci-api/app/returntypes"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/todo/auth/user"
	"github.com/golangci/golangci-api/pkg/todo/github"
	"github.com/golangci/golangci-api/pkg/todo/repos"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Service interface {
	//url:/v1/repos method:POST
	Create(rc *request.Context, reqRepo *request.BodyRepo) (*returntypes.RepoInfo, error)

	//url:/v1/repos/{repoID} method:DELETE
	//Delete(rc *request.Context, reqRepo *request.ProviderRepo) (*returntypes.RepoInfo, error)
}

type BasicService struct {
	DB *gorm.DB
}

func (s BasicService) Create(rc *request.Context, reqRepo *request.BodyRepo) (*returntypes.RepoInfo, error) {
	gc, _, err := github.GetClientV2(rc.Ctx, s.DB, nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't get github client")
	}

	githubRepo, _, err := gc.Repositories.Get(rc.Ctx, reqRepo.Owner, reqRepo.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo %s data", reqRepo)
	}

	if !githubRepo.GetPermissions()["admin"] {
		return nil, errors.New("no admin permission on repo")
	}

	var repo models.Repo
	if err = models.NewRepoQuerySet(s.DB).GithubIDEq(githubRepo.GetID()).One(&repo); err == nil {
		return nil, fmt.Errorf("attempt to activate already activated repo %s", repo.String())
	}

	hookID, err := utils.GenerateRandomString(32)
	if err != nil {
		return nil, errors.Wrap(err, "can't generate hook id")
	}

	ga, err := user.GetGithubAuthV2(s.DB, nil)
	if err != nil {
		return nil, errors.Wrap(err, "can't get current github auth")
	}

	gr := models.Repo{
		UserID:      ga.UserID,
		Name:        strings.ToLower(githubRepo.GetFullName()),
		DisplayName: githubRepo.GetFullName(),
		HookID:      hookID,
		Provider:    "github.com",
		GithubID:    githubRepo.GetID(),
	}

	web := "web"
	hookURL := os.Getenv("GITHUB_CALLBACK_HOST") + repos.GetWebhookURLPathForRepo(gr.Name, gr.HookID)
	hook := gh.Hook{
		Name:   &web,
		Events: []string{"push", "pull_request"},
		Config: map[string]interface{}{
			"url":          hookURL,
			"content_type": "json",
		},
	}
	rh, _, err := gc.Repositories.CreateHook(rc.Ctx, reqRepo.Owner, reqRepo.Name, &hook)
	if err != nil {
		return nil, errors.Wrapf(err, "can't post hook %v to github", hook)
	}

	gr.GithubHookID = rh.GetID()
	if err := gr.Create(s.DB); err != nil {
		return nil, errors.Wrap(err, "can't create repo")
	}

	return &returntypes.RepoInfo{
		Name:        gr.DisplayName,
		IsActivated: true,
		HookID:      gr.HookID,
	}, nil
}
