package auth

import (
	"github.com/golangci/golangci-api/pkg/apierrors"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/returntypes"
	"github.com/pkg/errors"
)

type Service interface {
	//url:/v1/auth/check
	CheckAuth(rc *request.AuthorizedContext) (*returntypes.CheckAuthResponse, error)

	//url:/v1/auth/logout
	Logout(rc *request.AuthorizedContext) error

	//url:/v1/auth/github/unlink method:PUT
	UnlinkProvider(rc *request.AuthorizedContext) error
}

type BasicService struct {
	WebRoot string
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

func (s BasicService) Logout(rc *request.AuthorizedContext) error {
	rc.Sess.Delete()
	return apierrors.NewTemporaryRedirectError(s.WebRoot + "?after=logout")
}

func (s BasicService) UnlinkProvider(rc *request.AuthorizedContext) (err error) {
	tx, finishTx, err := gormdb.StartTx(rc.DB)
	if err != nil {
		return err
	}
	defer finishTx(&err)

	if err = models.NewRepoQuerySet(tx).UserIDEq(rc.Auth.UserID).Delete(); err != nil {
		return errors.Wrap(err, "can't remove all repos")
	}

	if err = rc.Auth.Delete(tx.Unscoped()); err != nil {
		return errors.Wrap(err, "can't delete auth")
	}

	return nil
}
