package auth

import (
	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

const userIDSessKey = "UserID"
const SessType = "s"

func Get(authSess *session.Session, sctx *session.RequestContext, db *gorm.DB) (*models.Auth, error) {
	userIDobj := authSess.GetValue(userIDSessKey)
	if userIDobj == nil {
		return nil, apierrors.ErrNotAuthorized
	}

	userIDfloat := userIDobj.(float64)
	userID := uint(userIDfloat)

	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(userID).One(&auth); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.Wrapf(err, "no user with id %d", userID)
		}

		return nil, errors.Wrapf(err, "failed to fetch user with id %d", userID)
	}

	return &auth, nil
}

func Create(authSess *session.Session, userID uint) {
	authSess.Set(userIDSessKey, userID)
}
