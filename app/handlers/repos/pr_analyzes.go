package repos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	"github.com/jinzhu/gorm"
)

type State struct {
	gorm.Model

	Status                  string
	ReportedIssuesCount     int
	ResultJSON              json.RawMessage
	CommitSHA               string
	GithubPullRequestNumber int
	GithubRepoName          string
}

func handleAnalysisState(ctx context.C) error {
	switch ctx.R.Method {
	case http.MethodGet:
		return getAnalysisState(ctx)
	case http.MethodPut:
		return updateAnalysisState(ctx)
	default:
		return fmt.Errorf("not allowed method")
	}
}

func getAnalysisState(ctx context.C) error {
	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(db.Get(&ctx)).
		GithubDeliveryGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return db.Error(err, "can't get analysis with guid %s", analysisGUID)
	}

	repoName := fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name"))
	var repo models.Repo
	err = models.NewRepoQuerySet(db.Get(&ctx)).
		NameEq(repoName).
		One(&repo)
	if err != nil {
		return db.Error(err, "can't get repo %s", repoName)
	}

	ctx.ReturnJSON(State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          repo.Name,
	})
	return nil
}

func handlePRAnalysisState(ctx context.C) error {
	repoName := fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name"))
	var repo models.Repo
	err := models.NewRepoQuerySet(db.Get(&ctx)).
		NameEq(repoName).
		One(&repo)
	if err != nil {
		return db.Error(err, "can't get repo %s", repoName)
	}

	prNumber, err := strconv.Atoi(ctx.URLVar("prNumber"))
	if err != nil {
		return fmt.Errorf("invalid pr number %q: %s", ctx.URLVar("prNumber"), err)
	}

	var analysis models.PullRequestAnalysis
	err = models.NewPullRequestAnalysisQuerySet(db.Get(&ctx)).
		PullRequestNumberEq(prNumber).
		RepoIDEq(repo.ID).
		OrderDescByID(). // get last
		Limit(1).
		One(&analysis)
	if err != nil {
		return db.Error(err, "can't get pull request analysis with number %d and repo id %d", prNumber, repo.ID)
	}

	ctx.ReturnJSON(State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          repo.Name,
	})
	return nil
}

func updateAnalysisState(ctx context.C) error {
	var payload State
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return herrors.New400Errorf("invalid payload json: %s", err)
	}

	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(db.Get(&ctx)).
		GithubDeliveryGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get pull request analysis with guid %s", analysisGUID)
	}

	prevStatus := analysis.Status
	analysis.Status = payload.Status
	analysis.ReportedIssuesCount = payload.ReportedIssuesCount
	analysis.ResultJSON = payload.ResultJSON
	if analysis.ResultJSON == nil {
		analysis.ResultJSON = []byte("{}")
	}

	err = analysis.Update(db.Get(&ctx),
		models.PullRequestAnalysisDBSchema.Status,
		models.PullRequestAnalysisDBSchema.ReportedIssuesCount,
		models.PullRequestAnalysisDBSchema.ResultJSON)
	if err != nil {
		return herrors.New(err, "can't update pr analysis state for analytis %#v", analysis)
	}

	ctx.L.Infof("Updated analysis %s status: %s -> %s", analysisGUID, prevStatus, analysis.Status)
	return nil
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/analyzes/{analysisID}/state", handleAnalysisState)
	handlers.Register("/v1/repos/{owner}/{name}/pulls/{prNumber}", handlePRAnalysisState)

	handlers.Register("/v1/repos/github.com/{owner}/{name}/analyzes/{analysisID}/state", handleAnalysisState)
	handlers.Register("/v1/repos/github.com/{owner}/{name}/pulls/{prNumber}", handlePRAnalysisState)
}
