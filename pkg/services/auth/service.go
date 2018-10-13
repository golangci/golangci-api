package auth

import (
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/returntypes"
)

type Service interface {
	//url:/v1/auth/check method:GET
	CheckAuth(rc *request.AuthorizedContext) (*returntypes.CheckAuthResponse, error)
}

type BasicService struct{}

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
