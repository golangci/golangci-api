package repo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/api/util"
	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/golangci/golangci-api/pkg/api/returntypes"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/repos"
	"github.com/golangci/golangci-api/pkg/worker/lib/experiments"
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

	//url:/v1/repos/{repoid}
	Get(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error)

	//url:/v1/repos/{repoid} method:DELETE
	Delete(rc *request.AuthorizedContext, reqRepo *request.RepoID) (*returntypes.WrappedRepoInfo, error)

	//url:/v1/repos
	List(rc *request.AuthorizedContext, req *listRequest) (*returntypes.RepoListResponse, error)
}

type BasicService struct {
	CreateQueue     *repos.CreatorProducer
	DeleteQueue     *repos.DeleterProducer
	ProviderFactory providers.Factory
	Cache           cache.Cache
	Cfg             config.Config
	Ec              *experiments.Checker
	OrgPolicy       *policy.Organization
	ActiveSubPolicy *policy.ActiveSubscription
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
			ID:               repo.ID,
			Name:             repo.DisplayFullName,
			HookID:           repo.HookID,
			Organization:     repo.Owner(),
			IsAdmin:          true, // otherwise can't create or delete, it's checked
			IsDeleting:       repo.IsDeleting(),
			IsCreating:       repo.IsCreating(),
			IsActivated:      repo.DeletedAt == nil,
			CreateFailReason: repo.CreateFailReason,
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

func (s BasicService) canConnectPrivateRepos(pr *provider.Repo) bool {
	return s.Ec.IsActiveForRepo("CONNECT_PRIVATE_REPOS", pr.Owner(), pr.Name())
}

func (s BasicService) needSubscribe(rc *request.AuthorizedContext, org *models.Org, pr *provider.Repo) error {
	if !s.canConnectPrivateRepos(pr) {
		return apierrors.NewNotAcceptableError("NEED_PAID_MOCK")
	}

	errNeedToSubscribe := apierrors.NewContinueError(fmt.Sprintf("/orgs/%s/%s",
		rc.Auth.Provider, org.Name))

	rc.Log.Infof("Redirecting user to org settings")
	return errNeedToSubscribe
}

func (s BasicService) createOrganization(rc *request.AuthorizedContext, p provider.Provider, pr *provider.Repo) (*models.Org, error) {
	settings, jsonErr := json.Marshal(models.OrgSettings{
		Seats: []models.OrgSeat{
			{
				Email: rc.User.Email,
			},
		},
	})
	if jsonErr != nil {
		return nil, errors.Wrap(jsonErr, "failed to marshal json")
	}

	org := models.Org{
		Provider: p.Name(),
		Settings: json.RawMessage(settings),
	}
	var orgDisplayName string
	if pr.Organization == "" {
		org.ProviderPersonalUserID = pr.OwnerID
		orgDisplayName = pr.Owner()
	} else {
		org.ProviderID = pr.OwnerID
		orgDisplayName = pr.Organization
	}
	org.Name = strings.ToLower(orgDisplayName)
	org.DisplayName = orgDisplayName

	if err := s.OrgPolicy.CheckCanModify(rc, &org); err != nil {
		if err == policy.ErrNotOrgAdmin {
			err = policy.ErrNotOrgAdmin.
				WithMessage("The repo is private and there is no paid subscription: "+
					"subscription can be made only by the organization %s/%s admin",
					org.Provider, org.DisplayName)
		}
		if err == policy.ErrNotOrgMember {
			err = policy.ErrNotOrgAdmin.
				WithMessage("The repo is private and there is no paid subscription: "+
					"subscription can be made only by the organization %s/%s member",
					org.Provider, org.DisplayName)
		}

		return nil, err
	}

	if err := org.Create(rc.DB); err != nil {
		return nil, errors.Wrap(err, "can't create org")
	}

	return &org, nil
}

func (s BasicService) checkSubscription(rc *request.AuthorizedContext, pr *provider.Repo, p provider.Provider) error {
	if err := s.ActiveSubPolicy.CheckForProviderRepo(p, pr); err != nil {
		if err != policy.ErrNoActiveSubscription {
			return err
		}

		// continue: create organization if needed
	} else {
		return nil
	}

	// no active subscription: check does organization exist
	var org models.Org
	qs := models.NewOrgQuerySet(rc.DB).ForProviderRepo(p.Name(), pr.Organization, pr.OwnerID)
	if err := qs.One(&org); err != nil {
		if err != gorm.ErrRecordNotFound {
			return errors.Wrapf(err, "failed to fetch org from db for repo %s", pr.FullName)
		}

		if !s.canConnectPrivateRepos(pr) {
			return apierrors.NewNotAcceptableError("NEED_PAID_MOCK")
		}

		rc.Log.Infof("No org, creating it")
		createdOrg, createErr := s.createOrganization(rc, p, pr)
		if createErr != nil {
			return errors.Wrap(createErr, "failed to create organization")
		}
		org = *createdOrg
	}

	// org exists, but subscription isn't active
	return s.needSubscribe(rc, &org, pr)
}

func (s BasicService) Create(rc *request.AuthorizedContext, reqRepo *request.BodyRepo) (*returntypes.WrappedRepoInfo, error) {
	providerClient, err := s.ProviderFactory.Build(rc.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build provider")
	}

	if providerClient.Name() != reqRepo.Provider {
		return nil, fmt.Errorf("auth provider %s != request repo provider %s", providerClient.Name(), reqRepo.Provider)
	}

	providerRepo, err := providerClient.GetRepoByName(rc.Ctx, reqRepo.Owner, reqRepo.Name)
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

	if providerRepo.IsPrivate {
		if err = s.checkSubscription(rc, providerRepo, providerClient); err != nil {
			return nil, errors.Wrap(err, "check subscription")
		}
	}

	return s.storeRepo(rc, providerRepo)
}

func (s BasicService) storeRepo(rc *request.AuthorizedContext, providerRepo *provider.Repo) (*returntypes.WrappedRepoInfo, error) {
	hookID, err := util.GenerateRandomString(32)
	if err != nil {
		return nil, errors.Wrap(err, "can't generate hook id")
	}

	repo := models.Repo{
		UserID:          rc.Auth.UserID,
		FullName:        strings.ToLower(providerRepo.FullName),
		DisplayFullName: providerRepo.FullName,
		HookID:          hookID,
		Provider:        rc.Auth.Provider,
		ProviderID:      providerRepo.ID,
		CommitState:     models.RepoCommitStateCreateInit,
		StargazersCount: -1, // will be fetched later
		IsPrivate:       providerRepo.IsPrivate,
	}
	if err = repo.Create(rc.DB); err != nil {
		var existingRepo models.Repo
		exists := models.NewRepoQuerySet(rc.DB).ProviderIDEq(providerRepo.ID).One(&existingRepo) == nil
		if exists {
			rc.Log.Warnf("Race condition: failed to create repo, re-creating it: %s", err)
			return s.createAlreadyExistingRepo(rc, &existingRepo)
		}

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
		return nil, errors.Wrapf(err, "can't get repo %s from provider", repo.FullName)
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

	rc.Log.Infof("Sent repo %s to delete queue", repo.FullName)
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
			Name:         pr.FullName, // TODO: update changed name in models.Repo
			Organization: pr.Owner(),
			IsAdmin:      pr.IsAdmin,
			IsPrivate:    pr.IsPrivate,
			Language:     pr.Language,
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

	organizations, err := s.getOrganizationsInfo(rc, providerRepos)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get organizations")
	}

	return &returntypes.RepoListResponse{
		Repos:                   retRepos,
		PrivateRepos:            retPrivateRepos,
		PrivateReposWereFetched: rc.Auth.PrivateAccessToken != "",
		Organizations:           organizations,
	}, nil
}

func (s BasicService) fetchProviderReposCached(rc *request.AuthorizedContext, useCache bool, p provider.Provider) ([]provider.Repo, error) {
	const maxPages = 20
	key := fmt.Sprintf("repos/%s/fetch?user_id=%d&maxPage=%d&v=6", p.Name(), rc.Auth.UserID, maxPages)
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

func (s BasicService) getUserAccessibleOrgNames(repos []provider.Repo) map[string]bool {
	userAccessibleOrgNames := map[string]bool{}
	for _, repo := range repos {
		owner := repo.Owner()
		_, ok := userAccessibleOrgNames[owner]
		if !ok {
			userAccessibleOrgNames[owner] = repo.IsAdmin
			continue
		}

		if repo.IsAdmin {
			userAccessibleOrgNames[owner] = repo.IsAdmin
			continue
		}
	}

	return userAccessibleOrgNames
}

func (s BasicService) getAllSubscribedOrgs(db *gorm.DB) ([]models.Org, error) {
	var subs []models.OrgSub
	if err := models.NewOrgSubQuerySet(db).All(&subs); err != nil {
		return nil, errors.Wrap(err, "failed to fetch all subscriptions")
	}

	if len(subs) == 0 {
		return []models.Org{}, nil
	}

	var allSubscribedOrgIDs []uint
	for _, sub := range subs {
		allSubscribedOrgIDs = append(allSubscribedOrgIDs, sub.OrgID)
	}

	var allSubscribedOrgs []models.Org
	if err := models.NewOrgQuerySet(db).IDIn(allSubscribedOrgIDs...).All(&allSubscribedOrgs); err != nil {
		return nil, errors.Wrapf(err, "failed to fetch all %d orgs", len(allSubscribedOrgIDs))
	}

	return allSubscribedOrgs, nil
}

func (s BasicService) getOrganizationsInfo(rc *request.AuthorizedContext, repos []provider.Repo) (map[string]returntypes.OrgInfo, error) {
	allSubscribedOrgs, err := s.getAllSubscribedOrgs(rc.DB)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all subscribed orgs")
	}
	rc.Log.Infof("Have total %d subscribed orgs", len(allSubscribedOrgs))

	userAccessibleOrgNames := s.getUserAccessibleOrgNames(repos)
	rc.Log.Infof("User has %d accessible orgs: %v", len(userAccessibleOrgNames), userAccessibleOrgNames)

	ret := map[string]returntypes.OrgInfo{}
	for _, org := range allSubscribedOrgs {
		_, ok := userAccessibleOrgNames[org.Name] // must be already lower-cased
		if !ok {
			continue
		}

		if err := s.OrgPolicy.CheckCanModify(rc, &org); err != nil {
			if err == policy.ErrNotOrgAdmin || err == policy.ErrNotOrgMember {
				var reason string
				if err == policy.ErrNotOrgAdmin {
					reason = "Only organization admins can manage it's subscription"
				} else {
					reason = "Only organization members can manage it's subscription"
				}
				ret[org.Name] = returntypes.OrgInfo{
					HasActiveSubscription: true,
					CanModify:             false,
					CantModifyReason:      reason,
					Provider:              rc.Auth.Provider,
					Name:                  org.Name,
				}
				continue
			}

			rc.Log.Warnf("Failed to check access to org %s: %s", org.Name, err)
			continue
		}

		ret[org.Name] = returntypes.OrgInfo{
			HasActiveSubscription: true,
			CanModify:             true,
			Provider:              rc.Auth.Provider,
			Name:                  org.Name,
		}
	}

	rc.Log.Infof("User has access to %d orgs with subscriptions: %#v", len(ret), ret)

	for orgName, isAdmin := range userAccessibleOrgNames {
		if _, ok := ret[orgName]; ok {
			continue // org has subscription
		}

		ret[orgName] = returntypes.OrgInfo{
			HasActiveSubscription: false,
			CanModify:             isAdmin, // it's not accurate
			CantModifyReason:      "",
			Provider:              rc.Auth.Provider,
			Name:                  orgName,
		}
	}

	rc.Log.Infof("Organizations info: %#v", ret)
	return ret, nil
}
