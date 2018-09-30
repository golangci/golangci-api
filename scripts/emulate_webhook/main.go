package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/jinzhu/gorm"

	"github.com/satori/go.uuid"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golib/server/database"
	gh "github.com/google/go-github/github"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
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
		if err := emulatePullRequestWebhook(*repoName, *commitSHA, *hookID, *prNumber, *isProd); err != nil {
			log.Fatalf("Can't emulate pull_request webhook: %s", err)
		}
	case eventTypePush:
		if err := emulatePushWebhook(*repoName, *commitSHA, *branchName, *hookID, *isProd); err != nil {
			log.Fatalf("Can't emulate push webhook: %s", err)
		}
	default:
		log.Fatalf("unknown hook type %s", *hookType)
	}

	log.Printf("Successfully emulated webhook")
}

func emulatePullRequestWebhook(repoName, commitSHA, hookID string, prNumber int, isProd bool) error {
	payload := gh.PullRequestEvent{
		Action: gh.String("opened"),
		PullRequest: &gh.PullRequest{
			Number: gh.Int(prNumber),
			Head: &gh.PullRequestBranch{
				SHA: gh.String(commitSHA),
			},
		},
	}

	return sendWebhookPayload(repoName, eventTypePullRequest, hookID, isProd, payload)
}

func emulatePushWebhook(repoName, commitSHA, branchName, hookID string, isProd bool) error {
	payload := gh.PushEvent{
		Ref: gh.String(fmt.Sprintf("refs/heads/%s", branchName)),
		Repo: &gh.PushEventRepository{
			DefaultBranch: gh.String(branchName),
			FullName:      gh.String(repoName),
		},
		HeadCommit: &gh.PushEventCommit{
			ID: gh.String(commitSHA),
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
		repo, err := getOrCreateRepo(repoName)
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

	if resp.StatusCode != 200 {
		return fmt.Errorf("%q response status code %d", webhookURL, resp.StatusCode)
	}

	return nil
}

func getOrCreateRepo(repoName string) (*models.Repo, error) {
	var repo models.Repo
	err := models.NewRepoQuerySet(database.GetDB()).NameEq(repoName).One(&repo)
	if err == nil {
		return &repo, nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("can't get repo with name %q: %s", repoName, err)
	}

	// repo not found, create it
	var u models.User
	err = models.NewUserQuerySet(database.GetDB()).EmailEq("idenx@yandex.com").One(&u)
	if err != nil {
		return nil, fmt.Errorf("can't get user: %s", err)
	}

	repo.Name = repoName
	repo.DisplayName = repoName
	repo.UserID = u.ID
	repo.ProviderHookID = 1
	repo.HookID = uuid.NewV4().String()[:32]
	if err = repo.Create(database.GetDB()); err != nil {
		return nil, fmt.Errorf("can't create repo %#v: %s", repo, err)
	}

	logrus.Infof("created repo %#v", repo)
	return &repo, nil
}
