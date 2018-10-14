package oauth

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/endpoint/apierrors"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"

	"github.com/markbates/goth"
)

type Authorizer struct {
	providerName string
	provider     goth.Provider
	rand         *rand.Rand
	sessFactory  *session.Factory
	log          logutil.Log
}

func NewAuthorizer(providerName string, provider goth.Provider, sessFactory *session.Factory, log logutil.Log) *Authorizer {
	return &Authorizer{
		providerName: providerName,
		provider:     provider,
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		sessFactory:  sessFactory,
		log:          log,
	}
}

func (a Authorizer) buildSess(sctx *session.RequestContext) (*session.Session, error) {
	sess, err := a.sessFactory.Build(sctx, a.sessionName())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sess %s", a.sessionName())
	}

	return sess, nil
}

func (a Authorizer) RedirectToProvider(sctx *session.RequestContext) error {
	gothSess, err := a.provider.BeginAuth(a.generateState())
	if err != nil {
		return errors.Wrap(err, "failed to begin auth in goth provider")
	}

	url, err := gothSess.GetAuthURL()
	if err != nil {
		return errors.Wrap(err, "failed to get auth url from goth provider")
	}

	sess, err := a.buildSess(sctx)
	if err != nil {
		return err
	}

	sess.Set(a.providerName, gothSess.Marshal())

	a.log.Infof("Redirecting to provider %s url %q", a.providerName, url)
	return apierrors.NewTemporaryRedirectError(url)
}

type params struct {
	code string
}

func (p params) Get(s string) string {
	if s == "code" {
		return p.code
	}

	panic("requested unknown url param " + s)
}

func (a Authorizer) HandleProviderCallback(sctx *session.RequestContext, stateParam, codeParam string) (*goth.User, error) {
	sess, err := a.buildSess(sctx)
	if err != nil {
		return nil, err
	}
	defer sess.Delete()

	sessDataInterface := sess.GetValue(a.providerName)
	if sessDataInterface == nil {
		return nil, fmt.Errorf("could not find a matching session %q for this request in session %#v",
			a.providerName, sess)
	}
	sessData := sessDataInterface.(string)

	gothSess, err := a.provider.UnmarshalSession(sessData)
	if err != nil {
		return nil, errors.Wrap(err, "can't unmarshal to goth session")
	}

	if err = a.validateState(gothSess, stateParam); err != nil {
		return nil, errors.Wrap(err, "can't validate state")
	}

	// get access token
	p := params{
		code: codeParam,
	}
	if _, err = gothSess.Authorize(a.provider, p); err != nil {
		return nil, errors.Wrap(err, "can't authorize")
	}

	gu, err := a.provider.FetchUser(gothSess)
	if err != nil {
		return nil, errors.Wrap(err, "can't fetch user")
	}

	// Lowercase only email: don't lowercase nickname: it's not used as identifier anywhere
	gu.Email = strings.ToLower(gu.Email)

	return &gu, err
}

func (a Authorizer) sessionName() string {
	return a.providerName + "_oauth_sess"
}

func (a Authorizer) generateState() string {
	// If a state query param is not passed in, generate a random
	// base64-encoded nonce so that the state on the auth URL
	// is unguessable, preventing CSRF attacks, as described in
	//
	// https://auth0.com/docs/protocols/oauth2/oauth-state#keep-reading
	nonceBytes := make([]byte, 64)
	for i := 0; i < 64; i++ {
		nonceBytes[i] = byte(a.rand.Int63() % 256)
	}
	return base64.URLEncoding.EncodeToString(nonceBytes)
}

// validateState ensures that the state token param from the original
// AuthURL matches the one included in the current (callback) request.
func (a Authorizer) validateState(sess goth.Session, stateParam string) error {
	rawAuthURL, err := sess.GetAuthURL()
	if err != nil {
		return errors.Wrap(err, "failed to get auth url")
	}

	authURL, err := url.Parse(rawAuthURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse auth url %q", rawAuthURL)
	}

	originalState := authURL.Query().Get("state")
	if originalState != "" && (originalState != stateParam) {
		return fmt.Errorf("state token mismatch: %q != %q", originalState, stateParam)
	}

	return nil
}
