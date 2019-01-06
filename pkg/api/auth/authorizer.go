package auth

import (
	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

const userIDSessKey = "UserID"
const sessType = "s"

type Authorizer struct {
	db  *gorm.DB
	asf *session.Factory
}

func NewAuthorizer(db *gorm.DB, asf *session.Factory) *Authorizer {
	return &Authorizer{
		db:  db,
		asf: asf,
	}
}

type AuthenticatedUser struct {
	Auth     *models.Auth
	AuthSess *session.Session

	User *models.User
}

func (a Authorizer) Authorize(sctx *session.RequestContext) (*AuthenticatedUser, error) {
	authSess, err := a.asf.Build(sctx, sessType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build auth sess")
	}

	authModel, err := a.getAuthFromSession(authSess)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := models.NewUserQuerySet(a.db).IDEq(authModel.UserID).One(&user); err != nil {
		return nil, errors.Wrapf(err, "failed to fetch user %d from db", authModel.UserID)
	}

	return &AuthenticatedUser{
		Auth:     authModel,
		AuthSess: authSess,
		User:     &user,
	}, nil
}

func (a Authorizer) getAuthFromSession(authSess *session.Session) (*models.Auth, error) {
	userIDobj := authSess.GetValue(userIDSessKey)
	if userIDobj == nil {
		return nil, apierrors.ErrNotAuthorized
	}

	userIDfloat := userIDobj.(float64)
	userID := uint(userIDfloat)

	var auth models.Auth
	if err := models.NewAuthQuerySet(a.db).UserIDEq(userID).One(&auth); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.Wrapf(err, "no user with id %d", userID)
		}

		return nil, errors.Wrapf(err, "failed to fetch user with id %d", userID)
	}

	return &auth, nil
}

func (a Authorizer) CreateAuthorization(sctx *session.RequestContext, user *models.User) error {
	authSess, err := a.asf.Build(sctx, sessType)
	if err != nil {
		return errors.Wrap(err, "failed to build auth sess")
	}

	authSess.Set(userIDSessKey, user.ID)
	return nil
}
