package repos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
)

func handleRepoAnalysis(ctx context.C) error {
	switch ctx.R.Method {
	case http.MethodGet:
		return getRepoAnalysis(ctx)
	case http.MethodPut:
		return updateRepoAnalysis(ctx)
	default:
		return fmt.Errorf("not allowed method")
	}
}

func getRepoAnalysis(ctx context.C) error {
	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(&ctx)).
		AnalysisGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get repo analysis with guid %s", analysisGUID)
	}

	ctx.ReturnJSON(analysis)
	return nil
}

func updateRepoAnalysis(ctx context.C) error {
	var payload models.RepoAnalysis
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return herrors.New400Errorf("invalid payload json: %s", err)
	}

	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(&ctx)).
		AnalysisGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get repo analysis with guid %s", analysisGUID)
	}

	prevStatus := analysis.Status
	analysis.Status = payload.Status
	analysis.ResultJSON = payload.ResultJSON
	if analysis.ResultJSON == nil {
		analysis.ResultJSON = []byte("{}")
	}
	err = analysis.Update(db.Get(&ctx),
		models.RepoAnalysisDBSchema.Status,
		models.RepoAnalysisDBSchema.ResultJSON)
	if err != nil {
		return herrors.New(err, "can't update repo analysis")
	}

	ctx.L.Infof("Updated repo analysis %s state: status: %s -> %s", analysisGUID, prevStatus, analysis.Status)
	return nil
}

func handleRepoAnalyzesStatus(ctx context.C) error {
	repoName := fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name"))
	repoName = strings.ToLower(repoName)

	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(&ctx)).
		NameEq(repoName).
		One(&as)
	if err != nil {
		return db.Error(err, "can't get repo analysis status for %s", repoName)
	}

	var analysis models.RepoAnalysis
	err = models.NewRepoAnalysisQuerySet(db.Get(&ctx)).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID(). // get last
		Limit(1).
		One(&analysis)
	if err != nil {
		return db.Error(err, "can't get repo analysis with analysis status id %d", as.ID)
	}
	analysis.RepoAnalysisStatus = as

	resp := struct {
		models.RepoAnalysis
		GithubRepoName string
	}{
		RepoAnalysis:   analysis,
		GithubRepoName: repoName,
	}

	ctx.ReturnJSON(resp)
	return nil
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/repoanalyzes/{analysisID}", handleRepoAnalysis)
	handlers.Register("/v1/repos/{owner}/{name}/repoanalyzes", handleRepoAnalyzesStatus)
}
