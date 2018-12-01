package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/api/events"
	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/auth"
	"github.com/golangci/golangci-api/pkg/api/auth/oauth"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/golangci/golangci-api/pkg/api/returntypes"
	"github.com/jinzhu/gorm"
	"github.com/markbates/goth"
	"github.com/pkg/errors"
)

type Request struct {
	Provider string `request:",urlPart,"` // XXX: it's a short provider name e.g. 'github'
}

func (r Request) FillLogContext(lctx logutil.Context) {
	lctx["provider"] = r.Provider
}

type OAuthCallbackRequest struct {
	Request
	Code  string `request:",urlParam,"`
	State string `request:",urlParam,"`
}

type Service interface {
	//url:/v1/auth/check
	CheckAuth(rc *request.AuthorizedContext) (*returntypes.CheckAuthResponse, error)

	//url:/v1/auth/logout
	Logout(rc *request.AuthorizedContext) error

	//url:/v1/auth/unlink method:PUT
	UnlinkProvider(rc *request.AuthorizedContext) error

	//url:/v1/auth/user/relogin
	Relogin(rc *request.AuthorizedContext) error

	//url:/v1/auth/{provider}
	LoginPublic(rc *request.AnonymousContext, req *Request) error

	//url:/v1/auth/{provider}/private
	LoginPrivate(rc *request.AuthorizedContext, req *Request) error

	//url:/v1/auth/{provider}/callback/public
	LoginPublicOAuthCallback(rc *request.AnonymousContext, req *OAuthCallbackRequest) error

	//url:/v1/auth/{provider}/callback/private
	LoginPrivateOAuthCallback(rc *request.AuthorizedContext, req *OAuthCallbackRequest) error
}

type BasicService struct {
	Cfg             config.Config
	OAuthFactory    *oauth.Factory
	AuthSessFactory *session.Factory
}

func (s BasicService) CheckAuth(rc *request.AuthorizedContext) (*returntypes.CheckAuthResponse, error) {
	au := returntypes.AuthorizedUser{
		ID:          rc.User.ID,
		Name:        rc.User.Name,
		Email:       rc.User.Email,
		AvatarURL:   rc.User.AvatarURL,
		GithubLogin: rc.Auth.Login,
		CreatedAt:   rc.User.CreatedAt,
	}

	return &returntypes.CheckAuthResponse{
		User: au,
	}, nil
}

func (s BasicService) webroot() string {
	return s.Cfg.GetString("WEB_ROOT")
}

func (s BasicService) afterLogoutURL() string {
	return s.webroot() + "?after=logout"
}

func (s BasicService) afterLoginURL(provider string) string {
	return fmt.Sprintf("%s/repos/%s?after=login", s.webroot(), provider)
}

func (s BasicService) afterPrivateLoginURL(provider string) string {
	return fmt.Sprintf("%s/repos/%s?refresh=1&after=private_login", s.webroot(), provider)
}

func (s BasicService) Logout(rc *request.AuthorizedContext) error {
	rc.AuthSess.Delete()
	return apierrors.NewTemporaryRedirectError(s.afterLogoutURL())
}

func (s BasicService) UnlinkProvider(rc *request.AuthorizedContext) (retErr error) {
	tx, finishTx, err := gormdb.StartTx(rc.DB)
	if err != nil {
		return err
	}
	defer finishTx(&retErr)

	if err = models.NewRepoQuerySet(tx).UserIDEq(rc.Auth.UserID).Delete(); err != nil {
		return errors.Wrap(err, "can't remove all repos")
	}

	if err = rc.Auth.Delete(tx.Unscoped()); err != nil {
		return errors.Wrap(err, "can't delete auth")
	}

	return nil
}

func (s BasicService) login(rc *request.AnonymousContext, req *Request, isPrivate bool) error {
	authorizer, err := s.OAuthFactory.BuildAuthorizer(req.Provider, isPrivate)
	if err != nil {
		return errors.Wrap(err, "failed to build authorizer")
	}

	return authorizer.RedirectToProvider(rc.SessCtx)
}

func (s BasicService) LoginPublic(rc *request.AnonymousContext, req *Request) error {
	return s.login(rc, req, false)
}

func (s BasicService) LoginPrivate(rc *request.AuthorizedContext, req *Request) error {
	return s.login(rc.ToAnonumousContext(), req, true)
}

func (s BasicService) Relogin(rc *request.AuthorizedContext) error {
	provider := rc.Auth.Provider
	if provider == "github.com" {
		provider = "github" // TODO
	}
	req := &Request{
		Provider: provider,
	}
	if rc.Auth.PrivateAccessToken != "" {
		rc.Log.Infof("User has private access token: do private oauth relogin")
		return s.LoginPrivate(rc, req)
	}

	rc.Log.Infof("User has only public access token: do public oauth relogin")
	return s.LoginPublic(rc.ToAnonumousContext(), req)
}

func (s BasicService) LoginPublicOAuthCallback(rc *request.AnonymousContext, req *OAuthCallbackRequest) error {
	authorizer, err := s.OAuthFactory.BuildAuthorizer(req.Provider, false)
	if err != nil {
		return errors.Wrap(err, "failed to build authorizer")
	}

	gu, err := authorizer.HandleProviderCallback(rc.SessCtx, req.State, req.Code)
	if err != nil {
		return errors.Wrap(err, "can't handle public oauth callback")
	}

	rc.Log.Infof("%s public oauth completed, provider user is %+v, creating local user", req.Provider, gu)
	if err = s.LoginUser(rc, gu); err != nil {
		return errors.Wrap(err, "failed to login local user for provider user")
	}

	return apierrors.NewTemporaryRedirectError(s.afterLoginURL(req.Provider))
}

func (s BasicService) LoginPrivateOAuthCallback(rc *request.AuthorizedContext, req *OAuthCallbackRequest) error {
	authorizer, err := s.OAuthFactory.BuildAuthorizer(req.Provider, true)
	if err != nil {
		return errors.Wrap(err, "failed to build authorizer")
	}

	gu, err := authorizer.HandleProviderCallback(rc.SessCtx, req.State, req.Code)
	if err != nil {
		return errors.Wrap(err, "can't handle public oauth callback")
	}

	rc.Log.Infof("%s private oauth completed, updating access token", req.Provider)
	rc.Auth.PrivateAccessToken = gu.AccessToken
	if err = rc.Auth.Update(rc.DB, models.AuthDBSchema.PrivateAccessToken); err != nil {
		return errors.Wrap(err, "can't save private access token")
	}

	return apierrors.NewTemporaryRedirectError(s.afterPrivateLoginURL(req.Provider))
}

func (s BasicService) LoginUser(rc *request.AnonymousContext, gu *goth.User) (retErr error) {
	tx, finishTx, err := gormdb.StartTx(rc.DB)
	if err != nil {
		return err
	}
	defer finishTx(&retErr)

	u, gaID, err := getOrStoreUserInDB(rc.Ctx, rc.Log, tx, gu)
	if err != nil {
		return errors.Wrap(err, "failed to get or store user in db")
	}

	providerUserID, err := strconv.ParseUint(gu.UserID, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "can't parse provider user id %s", gu.UserID)
	}

	rawData, err := json.Marshal(gu.RawData)
	if err != nil {
		return errors.Wrap(err, "json marshal of raw data failed")
	}

	ga := models.Auth{
		Model: gorm.Model{
			ID: gaID,
		},
		RawData:        rawData,
		AccessToken:    gu.AccessToken,
		UserID:         u.ID,
		Login:          gu.NickName,
		Provider:       "github.com", // TODO
		ProviderUserID: providerUserID,
	}

	if gaID == 0 {
		if err = ga.Create(tx); err != nil {
			return errors.Wrapf(err, "can't create authorization %v", u)
		}
	} else {
		err = ga.Update(tx, models.AuthDBSchema.RawData,
			models.AuthDBSchema.AccessToken, models.AuthDBSchema.Login,
		)
		if err != nil {
			return errors.Wrapf(err, "can't update authorization %v", u)
		}
	}

	authSess, err := s.AuthSessFactory.Build(rc.SessCtx, auth.SessType)
	if err != nil {
		return errors.Wrap(err, "failed to build auth sess")
	}

	auth.Create(authSess, u.ID)
	return nil
}

func updateUserDataIfNeeded(log logutil.Log, tx *gorm.DB, u *models.User, gu *goth.User) error {
	var fieldsToUpdate []models.UserDBSchemaField
	if gu.Email != "" && u.Email != gu.Email {
		log.Infof("Updating user %d email from %s to %s on auth", u.ID, u.Email, gu.Email)
		u.Email = gu.Email
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.Email)
	}
	if gu.Name != "" && u.Name != gu.Name {
		log.Infof("Updating user %d name from %s to %s on auth", u.ID, u.Name, gu.Name)
		u.Name = gu.Name
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.Name)
	}
	if gu.AvatarURL != "" && u.AvatarURL != gu.AvatarURL {
		log.Infof("Updating user %d avatar from %s to %s on auth", u.ID, u.AvatarURL, gu.AvatarURL)
		u.AvatarURL = gu.AvatarURL
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.AvatarURL)
	}
	if len(fieldsToUpdate) != 0 {
		if err := u.Update(tx, fieldsToUpdate...); err != nil {
			return errors.Wrapf(err, "can't update user %d fields %v", u.ID, fieldsToUpdate)
		}
	}

	return nil
}

func getOrStoreUserInDB(ctx context.Context, log logutil.Log, tx *gorm.DB, gu *goth.User) (*models.User, uint, error) {
	var ga models.Auth
	providerUserID, err := strconv.ParseUint(gu.UserID, 10, 64)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "can't parse github user id %q", gu.UserID)
	}

	err = models.NewAuthQuerySet(tx).ProviderUserIDEq(providerUserID).OrderDescByID().One(&ga)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, errors.Wrapf(err, "can't select auth with provider user id %d", providerUserID)
	}

	if err == gorm.ErrRecordNotFound { // new user, need create it
		name := gu.Name
		if name == "" {
			name = gu.NickName
		}

		u := &models.User{
			Email:     gu.Email,
			Name:      name,
			AvatarURL: gu.AvatarURL,
		}
		if err = u.Create(tx); err != nil {
			return nil, 0, errors.Wrapf(err, "can't create user %v", u)
		}

		t := events.NewAuthenticatedTracker(int(u.ID)).WithUserProps(map[string]interface{}{
			"registeredAt": time.Now(),
		})

		go t.Track(ctx, "registered", map[string]interface{}{
			"provider": "github",
		})

		return u, 0, nil
	}

	var u models.User
	err = models.NewUserQuerySet(tx).IDEq(ga.UserID).One(&u)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "can't get user with id %d", ga.UserID)
	}

	if err = updateUserDataIfNeeded(log, tx, &u, gu); err != nil {
		return nil, 0, errors.Wrap(err, "can't update user data")
	}

	// User already exists
	return &u, ga.ID, nil
}
