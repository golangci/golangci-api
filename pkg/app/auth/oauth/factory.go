package oauth

import (
	"fmt"

	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/markbates/goth/providers/github"
)

type Factory struct {
	sessFactory *session.Factory
	log         logutil.Log
	cfg         config.Config
}

func NewFactory(sessFactory *session.Factory, log logutil.Log, cfg config.Config) *Factory {
	return &Factory{
		sessFactory: sessFactory,
		log:         log,
		cfg:         cfg,
	}
}

func (f Factory) BuildAuthorizer(providerName string, isPrivate bool) (*Authorizer, error) {
	if providerName != "github" {
		return nil, fmt.Errorf("provider %s isn't support for OAuth", providerName)
	}

	cbURL := fmt.Sprintf("/v1/auth/%s/callback/", providerName)
	if isPrivate {
		cbURL += "private"
		providerName += "_private"
	} else {
		cbURL += "public"
	}

	if isPrivate {
		provider := github.New(
			f.cfg.GetString("GITHUB_KEY"),
			f.cfg.GetString("GITHUB_SECRET"),
			f.cfg.GetString("GITHUB_CALLBACK_HOST")+cbURL,
			"user:email",
			"repo",
		)
		return NewAuthorizer(providerName, provider, f.sessFactory, f.log), nil
	}

	provider := github.New(
		f.cfg.GetString("GITHUB_KEY"),
		f.cfg.GetString("GITHUB_SECRET"),
		f.cfg.GetString("GITHUB_CALLBACK_HOST")+cbURL,
		"user:email",
		"public_repo",
	)
	return NewAuthorizer(providerName, provider, f.sessFactory, f.log), nil
}
