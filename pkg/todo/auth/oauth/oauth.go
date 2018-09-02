/*
Package gothic wraps common behaviour when using Goth. This makes it quick, and easy, to get up
and running with Goth. Of course, if you want complete control over how things flow, in regards
to the authentication process, feel free and use Goth directly.

See https://github.com/markbates/goth/examples/main.go to see this in action.
*/
package oauth

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golangci/golangci-api/pkg/todo/auth/sess"
	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
)

var store sessions.Store
var storeOnce sync.Once

var gothicRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type Authorizer struct {
	providerName string
	provider     goth.Provider
}

func (a Authorizer) RedirectToProvider(ctx *context.C) error {
	url, err := a.getAuthURL(ctx.W, ctx.R)
	if err != nil {
		return fmt.Errorf("can't get auth URL for OAuth: %s", err)
	}

	ctx.L.Infof("redirecting to %q", url)
	http.Redirect(ctx.W, ctx.R, url, http.StatusTemporaryRedirect)
	return nil
}

func (a Authorizer) getAuthURL(res http.ResponseWriter, req *http.Request) (string, error) {
	sess, err := a.provider.BeginAuth(setState(req))
	if err != nil {
		return "", err
	}

	url, err := sess.GetAuthURL()
	if err != nil {
		return "", err
	}

	if err = a.storeDataInSession(sess.Marshal(), req, res); err != nil {
		return "", err
	}

	return url, err
}

func (a Authorizer) HandleProviderCallback(ctx *context.C) (*goth.User, error) {
	value, err := a.getDataFromSession(ctx.R)
	if err != nil {
		return nil, fmt.Errorf("can't data get from session: %s", err)
	}

	defer func() {
		if cerr := a.Cleanup(ctx.W, ctx.R); cerr != nil {
			errors.Warnf(ctx, "Can't cleanup session: %s", cerr)
		}
	}()

	sess, err := a.provider.UnmarshalSession(value)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal session: %s", err)
	}

	if err = validateState(ctx.R, sess); err != nil {
		return nil, fmt.Errorf("can't validate state: %s", err)
	}

	// get access token
	if _, err = sess.Authorize(a.provider, ctx.R.URL.Query()); err != nil {
		return nil, fmt.Errorf("can't authorize: %s", err)
	}

	gu, err := a.provider.FetchUser(sess)
	if err != nil {
		return nil, fmt.Errorf("can't fetch user: %s", err)
	}

	// Don't lowercase nickname: it's not used as identifier anywhere
	gu.Email = strings.ToLower(gu.Email)

	return &gu, err
}

func (a Authorizer) sessionName() string {
	return a.providerName + "_oauth_sess"
}

// Logout invalidates a user session.
func (a Authorizer) Cleanup(res http.ResponseWriter, req *http.Request) error {
	session, err := getStore().Get(req, a.sessionName())
	if err != nil {
		return err
	}

	session.Options.MaxAge = -1
	session.Values = make(map[interface{}]interface{})
	if err = session.Save(req, res); err != nil {
		return herrors.New(err, "Could not delete user session ")
	}

	return nil
}

func (a Authorizer) storeDataInSession(value string, req *http.Request, res http.ResponseWriter) error {
	session, err := getStore().Get(req, a.sessionName())
	if err != nil {
		return fmt.Errorf("can't store in session %s: %s", a.sessionName(), err)
	}

	session.Values[a.providerName] = value
	return session.Save(req, res)
}

func (a Authorizer) getDataFromSession(req *http.Request) (string, error) {
	session, err := getStore().Get(req, a.sessionName())
	if err != nil {
		return "", fmt.Errorf("can't get session %s: %s", a.sessionName(), err)
	}

	value := session.Values[a.providerName]
	if value == nil {
		err = fmt.Errorf("could not find a matching session %q for this request in session %+v, cookie are %+v",
			a.providerName, session, req.Cookies())
		return "", err
	}

	return value.(string), nil
}

func getStore() sessions.Store {
	storeOnce.Do(func() {
		store = sess.CreateStore(3600) // 1 hour
	})
	return store
}

func setState(req *http.Request) string {
	state := req.URL.Query().Get("state")
	if len(state) > 0 {
		return state
	}

	// If a state query param is not passed in, generate a random
	// base64-encoded nonce so that the state on the auth URL
	// is unguessable, preventing CSRF attacks, as described in
	//
	// https://auth0.com/docs/protocols/oauth2/oauth-state#keep-reading
	nonceBytes := make([]byte, 64)
	for i := 0; i < 64; i++ {
		nonceBytes[i] = byte(gothicRand.Int63() % 256)
	}
	return base64.URLEncoding.EncodeToString(nonceBytes)
}

// validateState ensures that the state token param from the original
// AuthURL matches the one included in the current (callback) request.
func validateState(req *http.Request, sess goth.Session) error {
	rawAuthURL, err := sess.GetAuthURL()
	if err != nil {
		return err
	}

	authURL, err := url.Parse(rawAuthURL)
	if err != nil {
		return err
	}

	originalState := authURL.Query().Get("state")
	state := req.URL.Query().Get("state")
	if originalState != "" && (originalState != state) {
		return fmt.Errorf("state token mismatch: %q != %q", originalState, state)
	}
	return nil
}
