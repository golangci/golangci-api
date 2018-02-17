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
	"os"
	"sync"
	"time"

	"github.com/golangci/golangci-api/app/internal/auth/sess"
	"github.com/golangci/golib/server/handlers/herrors"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"

	log "github.com/sirupsen/logrus"
)

// SessionName is the key used to access the session store.
const sessionName = "github_oauth_sess"

var store sessions.Store
var storeOnce sync.Once

var gothicRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func getStore() sessions.Store {
	storeOnce.Do(func() {
		store = sess.CreateStore(3600) // 1 hour
	})
	return store
}

/*
BeginAuthHandler is a convenience handler for starting the authentication process.
It expects to be able to get the name of the provider from the query parameters
as either "provider" or ":provider".

BeginAuthHandler will redirect the user to the appropriate authentication end-point
for the requested provider.

See https://github.com/markbates/goth/examples/main.go to see this in action.
*/
func BeginAuthHandler(res http.ResponseWriter, req *http.Request) {
	url, err := GetAuthURL(res, req)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(res, err)
		return
	}

	log.Printf("redirecting to %q", url)
	http.Redirect(res, req, url, http.StatusTemporaryRedirect)
}

// SetState sets the state string associated with the given request.
// If no state string is associated with the request, one will be generated.
// This state is sent to the provider and can be retrieved during the
// callback.
var SetState = func(req *http.Request) string {
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

// GetState gets the state returned by the provider during the callback.
// This is used to prevent CSRF attacks, see
// http://tools.ietf.org/html/rfc6749#section-10.12
var GetState = func(req *http.Request) string {
	return req.URL.Query().Get("state")
}

/*
GetAuthURL starts the authentication process with the requested provided.
It will return a URL that should be used to send users to.

It expects to be able to get the name of the provider from the query parameters
as either "provider" or ":provider".

I would recommend using the BeginAuthHandler instead of doing all of these steps
yourself, but that's entirely up to you.
*/
func GetAuthURL(res http.ResponseWriter, req *http.Request) (string, error) {
	providerName, err := GetProviderName(req)
	if err != nil {
		return "", err
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return "", err
	}
	sess, err := provider.BeginAuth(SetState(req))
	if err != nil {
		return "", err
	}

	url, err := sess.GetAuthURL()
	if err != nil {
		return "", err
	}

	err = storeInSession(providerName, sess.Marshal(), req, res)
	if err != nil {
		return "", err
	}

	return url, err
}

/*
CompleteUserAuth does what it says on the tin. It completes the authentication
process and fetches all of the basic information about the user from the provider.

It expects to be able to get the name of the provider from the query parameters
as either "provider" or ":provider".

See https://github.com/markbates/goth/examples/main.go to see this in action.
*/
var CompleteUserAuth = func(res http.ResponseWriter, req *http.Request) (goth.User, error) {
	providerName, err := GetProviderName(req)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't get provider name: %s", err)
	}

	provider, err := goth.GetProvider(providerName)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't get provider: %s", err)
	}

	value, err := getFromSession(providerName, req)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't get from session: %s", err)
	}

	defer Logout(res, req)
	sess, err := provider.UnmarshalSession(value)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't unmarshal session: %s", err)
	}

	err = validateState(req, sess)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't validate state: %s", err)
	}

	// get access token
	_, err = sess.Authorize(provider, req.URL.Query())
	if err != nil {
		return goth.User{}, fmt.Errorf("can't authorize: %s", err)
	}

	gu, err := provider.FetchUser(sess)
	if err != nil {
		return goth.User{}, fmt.Errorf("can't fetch user: %s", err)
	}

	return gu, err
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

// Logout invalidates a user session.
func Logout(res http.ResponseWriter, req *http.Request) error {
	session, err := getStore().Get(req, sessionName)
	if err != nil {
		return err
	}
	session.Options.MaxAge = -1
	session.Values = make(map[interface{}]interface{})
	err = session.Save(req, res)
	if err != nil {
		return herrors.New(err, "Could not delete user session ")
	}
	return nil
}

// GetProviderName is a function used to get the name of a provider
// for a given request.
func GetProviderName(req *http.Request) (string, error) {
	return "github", nil
}

func storeInSession(key string, value string, req *http.Request, res http.ResponseWriter) error {
	session, _ := getStore().Get(req, sessionName)

	session.Values[key] = value

	return session.Save(req, res)
}

func getFromSession(key string, req *http.Request) (string, error) {
	session, _ := getStore().Get(req, sessionName)
	value := session.Values[key]
	if value == nil {
		return "", fmt.Errorf("could not find a matching session for this request")
	}

	return value.(string), nil
}

func InitGithub() {
	gh := github.New(
		os.Getenv("GITHUB_KEY"),
		os.Getenv("GITHUB_SECRET"),
		os.Getenv("GITHUB_CALLBACK_HOST")+"/v1/auth/github/callback",
		"user:email",
		"public_repo", // use "repo" to access private repos
	)
	log.Infof("Use github oauth: %+v", gh)
	goth.UseProviders(gh)
}

func init() {
	InitGithub()
}
