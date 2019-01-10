package oauth

import (
	"fmt"

	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
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

	key := f.cfg.GetString("GITHUB_KEY")
	secret := f.cfg.GetString("GITHUB_SECRET")
	cbHost := f.cfg.GetString("GITHUB_CALLBACK_HOST")

	if key == "" || secret == "" || cbHost == "" {
		return nil, fmt.Errorf("not all required GITHUB_* config params are set")
	}

	var scopes []string

	if isPrivate {
		scopes = []string{
			"user:email",
			"repo",
			//"read:org", // TODO(d.isaev): add it gracefully: save enabled grants to db and re-authorize only on needed page for needed users
		}
	} else {
		scopes = []string{
			"user:email",
			"public_repo",
		}
	}

	provider := github.New(
		key,
		secret,
		cbHost+cbURL,
		scopes...,
	)
	return NewAuthorizer(providerName, provider, f.sessFactory, f.log), nil
}
