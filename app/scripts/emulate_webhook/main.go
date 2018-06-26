package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golib/server/database"
	gh "github.com/google/go-github/github"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

func main() {
	repoName := flag.String("repo", "", "owner/name")
	commitSHA := flag.String("sha", "", "commit sha")
	prNumber := flag.Int("pr", 0, "pull request number")
	flag.Parse()

	if *repoName == "" || *commitSHA == "" || *prNumber == 0 {
		log.Fatalf("Must set --repo and --sha and --pr")
	}

	if err := emulateWebhook(*repoName, *commitSHA, *prNumber); err != nil {
		log.Fatalf("Can't emulate webhook: %s", err)
	}

	log.Printf("Successfully emulated webhook")
}

func emulateWebhook(repoName, commitSHA string, prNumber int) error {
	var repo models.GithubRepo
	err := models.NewGithubRepoQuerySet(database.GetDB()).NameEq(repoName).One(&repo)
	if err != nil {
		return fmt.Errorf("can't get repo with name %q: %s", repoName, err)
	}

	webhookURL := fmt.Sprintf("https://api.dev.golangci.com/v1/repos/%s/hooks/%s",
		repoName, repo.HookID)
	payload := gh.PullRequestEvent{
		Action: gh.String("opened"),
		PullRequest: &gh.PullRequest{
			Number: gh.Int(prNumber),
			Head: &gh.PullRequestBranch{
				SHA: gh.String(commitSHA),
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("can't marshal payload to json: %s", err)
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("can't create post request: %s", err)
	}

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
