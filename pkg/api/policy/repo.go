package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/pkg/api/auth"
	"github.com/golangci/golangci-api/pkg/api/request"

	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/pkg/errors"
)

var ErrNoProviderRepoOrAccess = errors.New("no provider repo or access to it")

type Repo struct {
	pf         providers.Factory
	cfg        config.Config
	log        logutil.Log
	cache      cache.Cache
	authorizer *auth.Authorizer
}

func NewRepo(pf providers.Factory, cfg config.Config, log logutil.Log, cache cache.Cache, authorizer *auth.Authorizer) *Repo {
	return &Repo{
		pf:         pf,
		cfg:        cfg,
		log:        log,
		cache:      cache,
		authorizer: authorizer,
	}
}

func (r Repo) CanRead(ctx context.Context, repo models.UniversalRepo, auth *models.Auth) error {
	type cachedResult struct {
		CanAccess bool
	}

	cacheKey := fmt.Sprintf("policy/repos/%s/%s/fetch?user_id=%d&v=1", repo.Owner(), repo.Repo(), auth.UserID)
	var cr cachedResult
	if err := r.cache.Get(cacheKey, &cr); err != nil {
		r.log.Warnf("Failed to fetch from cache by key %s: %s", cacheKey, err)
	} else if cr.CanAccess {
		r.log.Infof("Use cached info that user has access to repo %s/%s", repo.Owner(), repo.Repo())
		return nil
	}

	p, err := r.pf.Build(auth)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	_, err = p.GetRepoByName(ctx, repo.Owner(), repo.Repo())
	if err != nil {
		if err == provider.ErrNotFound {
			return ErrNoProviderRepoOrAccess
		}

		return errors.Wrap(err, "failed to get repo from provider")
	}

	const cacheTTL = time.Hour * 24 * 365 // 1 year
	if err := r.cache.Set(cacheKey, cacheTTL, cachedResult{CanAccess: true}); err != nil {
		r.log.Warnf("Failed to save to cache by key %s: %s", cacheKey, err)
	}

	r.log.Infof("User has access to repo %s/%s", repo.Owner(), repo.Repo())
	return nil
}

func (r Repo) CanReadPrivateRepo(rc *request.AnonymousContext, repo models.UniversalRepo) error {
	au, authErr := r.authorizer.Authorize(rc.SessCtx)
	if authErr != nil {
		if errors.Cause(authErr) == apierrors.ErrNotAuthorized {
			return apierrors.NewForbiddenError("NEED_AUTH_TO_ACCESS_PRIVATE_REPO")
		}

		return errors.Wrap(authErr, "failed to authorize")
	}

	if au.Auth.PrivateAccessToken == "" {
		return apierrors.NewForbiddenError("NEED_PRIVATE_ACCESS_TOKEN_TO_ACCESS_PRIVATE_REPO")
	}

	// TODO: make proper error if providers of repo and auth don't match
	if accessErr := r.CanRead(rc.Ctx, repo, au.Auth); accessErr != nil {
		if accessErr == ErrNoProviderRepoOrAccess {
			adminLogin := r.cfg.GetString("ADMIN_GITHUB_LOGIN")
			if adminLogin != "" && au.Auth.Provider == "github.com" && au.Auth.Login == adminLogin {
				r.log.Infof("Access repo %s as github admin user %s", repo.Owner(), repo.Repo(), adminLogin)
				return nil
			}

			return apierrors.NewForbiddenError("NO_ACCESS_TO_PRIVATE_REPO_OR_DOESNT_EXIST")
		}

		return errors.Wrap(accessErr, "failed to check read access to repo")
	}

	return nil
}
