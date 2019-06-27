package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/pkg/api/models"
	gh "github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

const (
	eventTypePullRequest = "pull_request"
	eventTypePush        = "push"
)

//nolint:gocyclo
func main() {
	repoName := flag.String("repo", "", "owner/name")
	hookType := flag.String("type", eventTypePullRequest, "hook type")
	commitSHA := flag.String("sha", "", "commit sha")
	prNumber := flag.Int("pr", 0, "pull request number")
	hookID := flag.String("hook-id", "", "Hook ID for repo")
	isProd := flag.Bool("prod", false, "is production")
	branchName := flag.String("branch", "master", "branch name")
	isPrivateRepo := flag.Bool("private", false, "is private repo")
	flag.Parse()

	if *repoName == "" || *commitSHA == "" {
		log.Fatalf("Must set --repo and --sha")
	}

	if *hookType == eventTypePullRequest && *prNumber == 0 {
		log.Fatalf("Must set --sha and --pr")
	}

	if *commitSHA == ":gen" {
		*commitSHA = "gen_" + uuid.NewV4().String()
	}

	switch *hookType {
	case eventTypePullRequest:
		if err := emulatePullRequestWebhook(*repoName, *commitSHA, *hookID, *prNumber, *isProd, *isPrivateRepo); err != nil {
			log.Fatalf("Can't emulate pull_request webhook: %s", err)
		}
	case eventTypePush:
		if err := emulatePushWebhook(*repoName, *commitSHA, *branchName, *hookID, *isProd, *isPrivateRepo); err != nil {
			log.Fatalf("Can't emulate push webhook: %s", err)
		}
	default:
		log.Fatalf("unknown hook type %s", *hookType)
	}

	log.Printf("Successfully emulated webhook")
}

func emulatePullRequestWebhook(repoName, commitSHA, hookID string, prNumber int, isProd, isPrivate bool) error {
	nameParts := strings.Split(repoName, "/")
	payload := gh.PullRequestEvent{
		Action: gh.String("opened"),
		PullRequest: &gh.PullRequest{
			Number: &prNumber,
			Head: &gh.PullRequestBranch{
				SHA: &commitSHA,
			},
		},
		Repo: &gh.Repository{
			FullName: &repoName,
			Private:  &isPrivate,
			Owner: &gh.User{
				Login: gh.String(nameParts[0]),
			},
			Name: gh.String(nameParts[1]),
		},
	}

	return sendWebhookPayload(repoName, eventTypePullRequest, hookID, isProd, payload)
}

func emulatePushWebhook(repoName, commitSHA, branchName, hookID string, isProd, isPrivate bool) error {
	if !isProd {
		log := logutil.NewStderrLog("")
		cfg := config.NewEnvConfig(log)
		db, err := gormdb.GetDB(cfg, log, "")
		if err != nil {
			return errors.Wrap(err, "failed to get gorm db")
		}

		if err = models.NewRepoAnalysisQuerySet(db.Unscoped()).CommitSHAEq(commitSHA).Delete(); err != nil {
			return errors.Wrapf(err, "failed to delete repo analyzes with commit SHA %s", commitSHA)
		}
		log.SetLevel(logutil.LogLevelInfo)
		log.Infof("Deleted repo analyzes with commit SHA %s", commitSHA)
	}

	payload := gh.PushEvent{
		Ref: gh.String(fmt.Sprintf("refs/heads/%s", branchName)),
		Repo: &gh.PushEventRepository{
			DefaultBranch: &branchName,
			FullName:      &repoName,
			Private:       &isPrivate,
		},
		HeadCommit: &gh.PushEventCommit{
			ID: &commitSHA,
		},
	}

	return sendWebhookPayload(repoName, eventTypePush, hookID, isProd, payload)
}

func sendWebhookPayload(repoName, event, hookID string, isProd bool, payload interface{}) error {
	var host string
	if isProd {
		host = "https://api.golangci.com"
	} else {
		host = "https://api.dev.golangci.com"
	}

	var webhookURL string
	if hookID != "" {
		webhookURL = fmt.Sprintf("%s/v1/repos/%s/hooks/%s", host, repoName, hookID)
	} else {
		repo, err := getOrCreateRepo(repoName, isProd)
		if err != nil {
			return fmt.Errorf("can't get/create repo %s: %s", repoName, err)
		}

		webhookURL = fmt.Sprintf("%s/v1/repos/%s/hooks/%s", host, repoName, repo.HookID)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("can't marshal payload to json: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("can't create post request: %s", err)
	}

	req.Header.Add("X-GitHub-Event", event)
	req.Header.Add("X-GitHub-Delivery", fmt.Sprintf("emulated_guid_%d", time.Now().UnixNano()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("can't send http request: %s", err)
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("%q response status code %d", webhookURL, resp.StatusCode)
	}

	return nil
}

func getOrCreateRepo(repoName string, isProd bool) (*models.Repo, error) {
	log := logutil.NewStderrLog("")
	cfg := config.NewEnvConfig(log)
	db, err := gormdb.GetDB(cfg, log, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get gorm db")
	}

	var repo models.Repo
	if err = models.NewRepoQuerySet(db).FullNameEq(repoName).One(&repo); err == nil {
		return &repo, nil
	}

	if isProd {
		return nil, fmt.Errorf("no repo, don't create the new one in prod mode")
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("can't get repo with name %q: %s", repoName, err)
	}

	// repo not found, create it
	var u models.User
	err = models.NewUserQuerySet(db).EmailEq("idenx@yandex.com").One(&u)
	if err != nil {
		return nil, fmt.Errorf("can't get user: %s", err)
	}

	repo.FullName = repoName
	repo.DisplayFullName = repoName
	repo.UserID = u.ID
	repo.ProviderHookID = 1
	repo.HookID = uuid.NewV4().String()[:32]
	repo.Provider = "github.com"
	repo.ProviderID = int(rand.Int31())
	repo.CommitState = models.RepoCommitStateCreateDone
	if err = repo.Create(db); err != nil {
		return nil, fmt.Errorf("can't create repo %#v: %s", repo, err)
	}

	logrus.Infof("created repo %#v", repo)
	return &repo, nil
}
