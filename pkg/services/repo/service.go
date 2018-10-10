package repo

import (
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golangci-api/pkg/cache"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/providers"
	"github.com/golangci/golangci-api/pkg/providers/provider"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/returntypes"
	"github.com/golangci/golangci-api/pkg/workers/primaryqueue/repos"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type listRequest struct {
	Refresh bool `request:",urlParam,optional"`
}

func (lr listRequest) FillLogContext(lctx logutil.Context) {
	lctx["refresh"] = lr.Refresh
}

type Service interface {
	//url:/v1/repos method:POST
	Create(rc *request.AuthorizedContext, reqRepo *request.BodyRepo) (*returntypes.WrappedRepoInfo, error)

	//url:/v1/repos/{repoid} method:GET
	Get(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error)

	//url:/v1/repos/{repoid} method:DELETE
	Delete(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error)

	//url:/v1/repos method:GET
	List(rc *request.AuthorizedContext, req *listRequest) (*returntypes.RepoListResponse, error)
}

type BasicService struct {
	CreateQueue     *repos.CreatorProducer
	DeleteQueue     *repos.DeleterProducer
	ProviderFactory *providers.Factory
	Cache           cache.Cache
	Cfg             config.Config
}

func (s BasicService) finishQueueSending(rc *request.AuthorizedContext, repo *models.Repo,
	expState, setState models.RepoCommitState) (*returntypes.WrappedRepoInfo, error) {

	n, err := models.NewRepoQuerySet(rc.DB).
		IDEq(repo.ID).CommitStateEq(expState).
		GetUpdater().
		SetCommitState(setState).
		UpdateNum()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update commit state to %s for repo with id %d",
			setState, repo.ID)
	}
	if n == 0 {
		rc.Log.Infof("Don't updating repo %#v commit state to %s because it's already updated by queue consumer",
			repo, setState)
	}
	repo.CommitState = setState

	return s.buildResponseRepo(repo), nil
}

func (s BasicService) sendToCreateQueue(rc *request.AuthorizedContext, repo *models.Repo) (*returntypes.WrappedRepoInfo, error) {
	if err := s.CreateQueue.Put(repo.ID); err != nil {
		return nil, errors.Wrap(err, "failed to put to create repos queue")
	}
	return s.finishQueueSending(rc, repo, models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue)
}

func (s BasicService) buildResponseRepo(repo *models.Repo) *returntypes.WrappedRepoInfo {
	return &returntypes.WrappedRepoInfo{
		Repo: returntypes.RepoInfo{
			ID:          repo.ID,
			Name:        repo.DisplayName,
			HookID:      repo.HookID,
			IsAdmin:     true, // otherwise can't create or delete, it's checked
			IsDeleting:  repo.IsDeleting(),
			IsCreating:  repo.IsCreating(),
			IsActivated: repo.DeletedAt == nil,
		},
	}
}

func (s BasicService) createAlreadyExistingRepo(rc *request.AuthorizedContext, repo *models.Repo) (*returntypes.WrappedRepoInfo, error) {
	switch repo.CommitState {
	case models.RepoCommitStateCreateInit:
		rc.Log.Warnf("Recreating repo with commit state %s, sending to queue: %#v",
			repo.CommitState, repo)
		return s.sendToCreateQueue(rc, repo)
	case models.RepoCommitStateCreateSentToQueue, models.RepoCommitStateCreateCreatedRepo, models.RepoCommitStateCreateDone:
		rc.Log.Warnf("Recreating repo with commit state %s, return ok: %#v",
			repo.CommitState, repo)
		return s.buildResponseRepo(repo), nil
	}

	return nil, fmt.Errorf("invalid repo commit state %s", repo.CommitState)
}

func (s BasicService) Create(rc *request.AuthorizedContext, reqRepo *request.BodyRepo) (*returntypes.WrappedRepoInfo, error) {
	provider, err := s.ProviderFactory.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	if provider.Name() != reqRepo.Provider {
		return nil, fmt.Errorf("auth provider %s != request repo provider %s", provider.Name(), reqRepo.Provider)
	}

	providerRepo, err := provider.GetRepoByName(rc.Ctx, reqRepo.Owner, reqRepo.Name)
	if err != nil {
		//TODO: handle case when repo was removed (but not made private)
		return nil, errors.Wrapf(err, "can't get repo %s from provider", reqRepo)
	}

	if !providerRepo.IsAdmin {
		return nil, errors.New("no admin permission on repo")
	}

	var repo models.Repo
	err = models.NewRepoQuerySet(rc.DB).ProviderIDEq(providerRepo.ID).One(&repo)
	if err == nil {
		return s.createAlreadyExistingRepo(rc, &repo)
	}
	if err != gorm.ErrRecordNotFound {
		return nil, errors.Wrap(err, "failed to fetch repo from db")
	}

	hookID, err := utils.GenerateRandomString(32)
	if err != nil {
		return nil, errors.Wrap(err, "can't generate hook id")
	}

	repo = models.Repo{
		UserID:      rc.Auth.UserID,
		Name:        strings.ToLower(providerRepo.Name),
		DisplayName: providerRepo.Name,
		HookID:      hookID,
		Provider:    provider.Name(),
		ProviderID:  providerRepo.ID,
		CommitState: models.RepoCommitStateCreateInit,
	}
	if err = repo.Create(rc.DB); err != nil {
		return nil, errors.Wrap(err, "can't create repo")
	}

	ret, err := s.sendToCreateQueue(rc, &repo)
	if err != nil {
		return nil, err
	}

	rc.Log.Infof("Created repo %#v", repo)
	return ret, nil
}

func (s BasicService) Get(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error) {
	var repo models.Repo
	if err := models.NewRepoQuerySet(rc.DB.Unscoped()).IDEq(reqRepo.ID).One(&repo); err != nil {
		return nil, errors.Wrapf(err, "failed to to get repo from db with id %d", reqRepo.ID)
	}

	return s.buildResponseRepo(&repo), nil
}

func (s BasicService) Delete(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error) {
	var repo models.Repo
	err := models.NewRepoQuerySet(rc.DB.Unscoped()).IDEq(reqRepo.ID).ProviderEq(rc.Auth.Provider).One(&repo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch repo from db")
	}

	if repo.DeletedAt != nil {
		rc.Log.Warnf("Repo is already deleted")
		return s.buildResponseRepo(&repo), nil
	}

	switch repo.CommitState {
	case models.RepoCommitStateCreateDone:
		break // normal case: not being deleted now repo
	case models.RepoCommitStateDeleteInit:
		rc.Log.Warnf("Redeleting repo with commit state %s, sending to queue: %#v",
			repo.CommitState, repo)
		return s.sendToDeleteQueue(rc, &repo)
	case models.RepoCommitStateDeleteSentToQueue:
		rc.Log.Warnf("Redeleting repo with commit state %s, return ok: %#v",
			repo.CommitState, repo)
		return s.buildResponseRepo(&repo), nil
	default:
		return nil, fmt.Errorf("invalid repo commit state %s", repo.CommitState)
	}

	return s.startDelete(rc, &repo)
}

func (s BasicService) startDelete(rc *request.AuthorizedContext, repo *models.Repo) (*returntypes.WrappedRepoInfo, error) {
	provider, err := s.ProviderFactory.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	providerRepo, err := provider.GetRepoByName(rc.Ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo %s from provider", repo.Name)
	}

	if !providerRepo.IsAdmin {
		return nil, errors.New("no admin permission on repo")
	}

	n, err := models.NewRepoQuerySet(rc.DB).IDEq(repo.ID).CommitStateEq(models.RepoCommitStateCreateDone).
		GetUpdater().SetCommitState(models.RepoCommitStateDeleteInit).UpdateNum()
	if err != nil {
		return nil, errors.Wrap(err, "can't update repo commit state")
	}
	if n != 1 {
		return nil, fmt.Errorf("race condition during update repo with id %d, n=%d, repo=%#v", repo.ID, n, repo)
	}

	ret, err := s.sendToDeleteQueue(rc, repo)
	if err != nil {
		return nil, err
	}

	rc.Log.Infof("Deleted repo %s", repo.Name)
	return ret, nil
}

func (s BasicService) sendToDeleteQueue(rc *request.AuthorizedContext, repo *models.Repo) (*returntypes.WrappedRepoInfo, error) {
	// It's important to send rc.Auth.UserID to queue because it can differ from repo.UserID
	if err := s.DeleteQueue.Put(repo.ID, rc.Auth.UserID); err != nil {
		return nil, errors.Wrap(err, "failed to put to delete repos queue")
	}

	return s.finishQueueSending(rc, repo, models.RepoCommitStateDeleteInit, models.RepoCommitStateDeleteSentToQueue)
}

func (s BasicService) List(rc *request.AuthorizedContext, req *listRequest) (*returntypes.RepoListResponse, error) {
	provider, err := s.ProviderFactory.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	providerRepos, err := s.fetchProviderReposCached(rc, !req.Refresh, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch repos from provider")
	}

	activatedRepos, err := s.getActivatedRepos(rc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get activated repos")
	}

	retRepos := []returntypes.RepoInfo{}
	retPrivateRepos := []returntypes.RepoInfo{}
	for _, pr := range providerRepos {
		retRepo := returntypes.RepoInfo{
			Name:      pr.Name, // TODO: update changed name in models.Repo
			IsAdmin:   pr.IsAdmin,
			IsPrivate: pr.IsPrivate,
		}

		if ar, ok := activatedRepos[pr.ID]; pr.ID != 0 && ok {
			retRepo.ID = ar.ID
			retRepo.HookID = ar.HookID
			retRepo.IsActivated = true // activated by ANY user
			retRepo.IsDeleting = ar.IsDeleting()
			retRepo.IsCreating = ar.IsCreating()
		}
		if retRepo.IsPrivate {
			retPrivateRepos = append(retPrivateRepos, retRepo)
		} else {
			retRepos = append(retRepos, retRepo)
		}
	}

	return &returntypes.RepoListResponse{
		Repos:                   retRepos,
		PrivateRepos:            retPrivateRepos,
		PrivateReposWereFetched: rc.Auth.PrivateAccessToken != "",
	}, nil
}

func (s BasicService) fetchProviderReposCached(rc *request.AuthorizedContext, useCache bool, p provider.Provider) ([]provider.Repo, error) {
	const maxPages = 20
	key := fmt.Sprintf("repos/%s/fetch?user_id=%d&maxPage=%d&v=4", p.Name(), rc.Auth.UserID, maxPages)
	if rc.Auth.PrivateAccessToken != "" {
		key += "&private=true"
	}

	var repos []provider.Repo
	if useCache {
		if err := s.Cache.Get(key, &repos); err != nil {
			rc.Log.Warnf("Can't fetch repos from cache by key %s: %s", key, err)
			return s.fetchProviderReposFromProvider(rc, p, maxPages)
		}

		if len(repos) != 0 {
			rc.Log.Infof("Returning %d repos from cache", len(repos))
			return repos, nil
		}

		rc.Log.Infof("No repos in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup repos in cache, refreshing repos from provider...")
	}

	var err error
	repos, err = s.fetchProviderReposFromProvider(rc, p, maxPages)
	if err != nil {
		return nil, err
	}

	cacheTTL := s.Cfg.GetDuration("REPOS_CACHE_TTL", time.Hour*24*7)
	if err = s.Cache.Set(key, cacheTTL, repos); err != nil {
		rc.Log.Warnf("Can't save %d repos to cache by key %s: %s", len(repos), key, err)
	}

	return repos, nil
}

func (s BasicService) fetchProviderReposFromProvider(rc *request.AuthorizedContext, p provider.Provider, maxPages int) ([]provider.Repo, error) {
	// list all repositories for the authenticated user
	visibility := "public"
	if rc.Auth.PrivateAccessToken != "" {
		visibility = "all"
	}

	repos, err := p.ListRepos(rc.Ctx, &provider.ListReposConfig{
		Visibility: visibility,
		Sort:       "pushed",
		MaxPages:   maxPages,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch repos from provider %s", p.Name())
	}

	return repos, nil
}

func (s BasicService) getActivatedRepos(rc *request.AuthorizedContext) (map[int]models.Repo, error) {
	startedAt := time.Now()

	var repos []models.Repo
	if err := models.NewRepoQuerySet(rc.DB).All(&repos); err != nil {
		return nil, errors.Wrap(err, "can't select activated repos from db")
	}

	ret := map[int]models.Repo{}
	for _, r := range repos {
		ret[r.ProviderID] = r
	}

	rc.Log.Infof("Built map of all %d activated repos for %s", len(ret), time.Since(startedAt))
	if len(ret) < 10 {
		rc.Log.Infof("Repos map: %#v", ret)
	}

	return ret, nil
}
