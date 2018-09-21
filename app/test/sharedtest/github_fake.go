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
	"path/filepath"
	"strings"
	"sync"

	"github.com/golangci/golangci-api/app/utils"
	"github.com/gorilla/mux"
	"github.com/markbates/goth/providers/github"
)

type FakeGithubServerConfig struct {
	AuthHandler          http.HandlerFunc
	TokenHandler         http.HandlerFunc
	ProfileHandler       http.HandlerFunc
	GetRepoHandler       http.HandlerFunc
	ListRepoHooksHandler http.HandlerFunc
	GetBranchHandler     http.HandlerFunc
	EmailHandler         http.HandlerFunc
	ListReposHandler     http.HandlerFunc
	AddHookHandler       http.HandlerFunc
	DeleteHookHandler    http.HandlerFunc
}

var FakeGithubCfg = FakeGithubServerConfig{
	AuthHandler:          authHandler,
	TokenHandler:         tokenHandler,
	ProfileHandler:       profileHandler,
	GetRepoHandler:       getRepoHandler,
	ListRepoHooksHandler: listRepoHooksHandler,
	GetBranchHandler:     getBranchHandler,
	EmailHandler:         emailHandler,
	ListReposHandler:     listReposHandler,
	AddHookHandler:       addHookHandler,
	DeleteHookHandler:    deleteHookHandler,
}
var fakeGithubServer *httptest.Server
var fakeGithubServerOnce sync.Once

func wrapF(f *http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		curF := *f
		curF(w, r)
	}
}

func initFakeGithubServer() {
	r := mux.NewRouter()
	r.HandleFunc(authURL, FakeGithubCfg.AuthHandler)
	r.HandleFunc(tokenURL, FakeGithubCfg.TokenHandler)
	r.HandleFunc(profileURL, wrapF(&FakeGithubCfg.ProfileHandler))
	r.HandleFunc("/repos/{owner}/{repo}", FakeGithubCfg.GetRepoHandler)
	r.Methods("GET").Path("/repos/{owner}/{repo}/hooks").HandlerFunc(FakeGithubCfg.ListRepoHooksHandler)
	r.Methods("POST").Path("/repos/{owner}/{repo}/hooks").HandlerFunc(FakeGithubCfg.AddHookHandler)
	r.HandleFunc("/repos/{owner}/{repo}/branches/{branch}", FakeGithubCfg.GetBranchHandler)
	r.HandleFunc(emailURL, FakeGithubCfg.EmailHandler)
	r.HandleFunc("/user/repos", FakeGithubCfg.ListReposHandler)
	r.HandleFunc("/repos/{owner}/{repo}/hooks/{hookID}", FakeGithubCfg.DeleteHookHandler)

	fakeGithubServer = httptest.NewServer(r)

	github.AuthURL = fakeGithubServer.URL + authURL
	github.TokenURL = fakeGithubServer.URL + tokenURL
	github.ProfileURL = fakeGithubServer.URL + profileURL
	github.EmailURL = fakeGithubServer.URL + emailURL

	os.Setenv("GITHUB_CALLBACK_HOST", server.URL)
	os.Setenv("WEB_ROOT", server.URL)
}

const (
	authURL    = "/login/oauth/authorize"
	tokenURL   = "/login/oauth/access_token" //nolint:gas
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

func SendJSON(w http.ResponseWriter, obj interface{}) {
	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		log.Fatalf("Can't JSON encode result: %s", err)
	}
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	ret := map[string]string{
		"access_token": "valid_access_token",
	}
	SendJSON(w, ret)
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	sendFakeGithubJSON("get_profile", w)
}

func getRepoHandler(w http.ResponseWriter, r *http.Request) {
	fileName := strings.ToLower(filepath.Join("get_repo", mux.Vars(r)["owner"], mux.Vars(r)["repo"]))
	sendFakeGithubJSON(fileName, w)
}

func listRepoHooksHandler(w http.ResponseWriter, r *http.Request) {
	fileName := strings.ToLower(filepath.Join("get_repo_hooks", mux.Vars(r)["owner"], mux.Vars(r)["repo"]))
	sendFakeGithubJSON(fileName, w)
}

func getBranchHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	fileName := strings.ToLower(filepath.Join("get_branch", v["owner"], v["repo"], v["branch"]))
	sendFakeGithubJSON(fileName, w)
}

func emailHandler(w http.ResponseWriter, r *http.Request) {
}

func listReposHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if (q.Get("visibility") != "public" && q.Get("visibility") != "all") || q.Get("sort") != "pushed" {
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
