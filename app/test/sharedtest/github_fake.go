package sharedtest

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sync"

	"github.com/golangci/golangci-api/app/internal/auth/oauth"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/gorilla/mux"
	"github.com/markbates/goth/providers/github"
)

var fakeGithubServer *httptest.Server
var fakeGithubServerOnce sync.Once

const (
	authURL    = "/login/oauth/authorize"
	tokenURL   = "/login/oauth/access_token"
	profileURL = "/user"
	emailURL   = "/user/emails"
)

func authHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ru := fmt.Sprintf("%s?state=%s", q.Get("redirect_uri"), q.Get("state"))
	http.Redirect(w, r, ru, http.StatusTemporaryRedirect)
}

func sendFakeGithubJSON(name string, w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	f, err := os.Open(path.Join(utils.GetProjectRoot(), "app", "test", "data", "github_fake_response", name+".json"))
	if err != nil {
		log.Fatalf("Can't open data file %s: %s", name, err)
	}
	if _, err = io.Copy(w, f); err != nil {
		log.Fatalf("Can't write data file to output: %s", err)
	}
}

func sendJSON(w http.ResponseWriter, obj interface{}) {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		log.Fatalf("Can't JSON encode result: %s", err)
	}
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	ret := map[string]string{
		"access_token": "valid_access_token",
	}
	sendJSON(w, ret)
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	sendFakeGithubJSON("get_profile", w)
}

func emailHandler(w http.ResponseWriter, r *http.Request) {
}

func listReposHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("visibility") != "public" || q.Get("sort") != "pushed" {
		log.Printf("Invalid query params: %+v", q)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	page := q.Get("page")
	if page == "" {
		page = "1"
	}

	link := ""
	switch page {
	case "1":
		link = `<https://api.github.com/user/repos?access_token=xxx&sort=pushed&visibility=public&page=2>; rel="next", <https://api.github.com/user/repos?access_token=xxx&sort=pushed&visibility=public&page=2>; rel="last"`
	case "2":
		link = `<https://api.github.com/user/repos?access_token=xxx&sort=pushed&visibility=public&page=1>; rel="prev", <https://api.github.com/user/repos?access_token=xxx&sort=pushed&visibility=public&page=1>; rel="first"`
	}
	w.Header().Add("Link", link)

	sendFakeGithubJSON("list_repo_page"+page, w)
}

func addHookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sendFakeGithubJSON("add_hook", w)
}

func deleteHookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func initFakeGithubServer() {
	r := mux.NewRouter()
	r.HandleFunc(authURL, authHandler)
	r.HandleFunc(tokenURL, tokenHandler)
	r.HandleFunc(profileURL, profileHandler)
	r.HandleFunc(emailURL, emailHandler)
	r.HandleFunc("/user/repos", listReposHandler)
	r.HandleFunc("/repos/{owner}/{repo}/hooks", addHookHandler)
	r.HandleFunc("/repos/{owner}/{repo}/hooks/{hookID}", deleteHookHandler)

	fakeGithubServer = httptest.NewServer(r)

	github.AuthURL = fakeGithubServer.URL + authURL
	github.TokenURL = fakeGithubServer.URL + tokenURL
	github.ProfileURL = fakeGithubServer.URL + profileURL
	github.EmailURL = fakeGithubServer.URL + emailURL

	os.Setenv("GITHUB_CALLBACK_HOST", server.URL)
	os.Setenv("WEB_ROOT", server.URL)

	oauth.InitGithub()
}
