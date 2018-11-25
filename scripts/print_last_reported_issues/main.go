package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/golangci/golangci-lint/pkg/result"

	"github.com/golangci/golangci-lint/pkg/printers"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

func main() {
	if err := printLastReportedIssues(); err != nil {
		log.Fatalf("Failed to print last reported issues: %s", err)
	}
}

type pull struct {
	repo     *models.Repo
	analyzes []models.PullRequestAnalysis
}

func printLastReportedIssues() error {
	log := logutil.NewStderrLog("")
	log.SetLevel(logutil.LogLevelInfo)
	cfg := config.NewEnvConfig(log)
	db, err := gormdb.GetDB(cfg, log, "")
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	duration := cfg.GetDuration("REPORT_DURATION", 3*24*time.Hour)
	analyzeAfterCreatedAt := time.Now().Add(-duration)

	var pullAnalyzes []models.PullRequestAnalysis
	err = models.NewPullRequestAnalysisQuerySet(db).
		CreatedAtGt(analyzeAfterCreatedAt).
		OrderDescByCreatedAt().
		All(&pullAnalyzes)
	if err != nil {
		return errors.Wrap(err, "failed to get pull analyzes")
	}

	pullToAnalyzes := map[string]*pull{}
	for _, a := range pullAnalyzes {
		pullID := fmt.Sprintf("%d#%d", a.RepoID, a.PullRequestNumber)
		if pullToAnalyzes[pullID] == nil {
			pullToAnalyzes[pullID] = &pull{}
		}
		p := pullToAnalyzes[pullID]

		if p.repo == nil {
			var repo models.Repo
			if err = models.NewRepoQuerySet(db.Unscoped()).IDEq(a.RepoID).One(&repo); err != nil {
				return errors.Wrapf(err, "failed to get repo %d", a.RepoID)
			}
			p.repo = &repo
		}

		p.analyzes = append(p.analyzes, a)
	}

	log.Infof("Fetched %d analyzes for %d pull requests", len(pullAnalyzes), len(pullToAnalyzes))

	for _, pull := range pullToAnalyzes {
		if err = printPullIssues(log, pull); err != nil {
			log.Warnf("Failed to print pull issues: %s", err)
		}
	}

	return nil
}

func printPullIssues(log logutil.Log, pull *pull) error {
	uniqIssues := map[string]bool{}

	for _, a := range pull.analyzes {
		issues, err := getAnalysisIssues(&a)
		if err != nil {
			return errors.Wrapf(err, "failed to get pull %s#%d issues", pull.repo.Name, a.PullRequestNumber)
		}

		for _, i := range issues {
			issueID := fmt.Sprintf("%s:%d:%d: %s (%s)", i.FilePath(), i.Line(), i.Column(), i.Text, i.FromLinter)
			if uniqIssues[issueID] {
				continue
			}
			uniqIssues[issueID] = true
		}
	}

	lastAnalysis := pull.analyzes[0]
	if lastAnalysis.Status == "processed/success" && len(uniqIssues) == 0 {
		return nil
	}

	log.Infof("https://github.com/%s/pull/%d - %d uniq issues, last analysis status is %s",
		pull.repo.DisplayName, lastAnalysis.PullRequestNumber, len(uniqIssues), lastAnalysis.Status)
	for issueID := range uniqIssues {
		log.Infof("    %s", issueID)
	}
	log.Infof("----")

	return nil
}

func getAnalysisIssues(a *models.PullRequestAnalysis) ([]result.Issue, error) {
	if a.ResultJSON == nil {
		return nil, errors.New("no result json")
	}

	var res struct {
		GolangciLintRes printers.JSONResult
	}
	if err := json.Unmarshal(a.ResultJSON, &res); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json")
	}

	return res.GolangciLintRes.Issues, nil
}
