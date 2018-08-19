package repos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

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

func isCompleteAnalysisStatus(s string) bool {
	return s == "processed" || s == "error"
}

func handleRepoAnalyzesStatus(ctx context.C) error {
	type response struct {
		models.RepoAnalysis
		GithubRepoName     string
		NextAnalysisStatus string `json:",omitempty"`
		IsPreparing        bool   `json:",omitempty"`
	}

	repoName := fmt.Sprintf("%s/%s", ctx.URLVar("owner"), ctx.URLVar("name"))
	repoName = strings.ToLower(repoName)

	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(db.Get(&ctx)).
		NameEq(repoName).
		One(&as)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			n, _ := models.NewGithubRepoQuerySet(db.Get(&ctx)).NameEq(repoName).Count()
			if n != 0 {
				resp := response{
					IsPreparing:    true,
					GithubRepoName: repoName,
				}
				ctx.ReturnJSON(resp)
				return nil
			}
		}
		return db.Error(err, "can't get repo analysis status for %s", repoName)
	}

	var analyzes []models.RepoAnalysis
	err = models.NewRepoAnalysisQuerySet(db.Get(&ctx)).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID(). // get last
		Limit(2).
		All(&analyzes)
	if err != nil || len(analyzes) == 0 {
		return db.Error(err, "can't get repo analyzes with analysis status id %d", as.ID)
	}

	var lastCompleteAnalysis models.RepoAnalysis
	var nextAnalysisStatus string

	if !isCompleteAnalysisStatus(analyzes[0].Status) { // the last analysis is running now
		if len(analyzes) == 1 || !isCompleteAnalysisStatus(analyzes[1].Status) {
			// render that analysis is running (yes, it's not complete)
			lastCompleteAnalysis = analyzes[0]
		} else {
			// prev analysis was complete, render it and show that new analysis is running
			lastCompleteAnalysis = analyzes[1]
			nextAnalysisStatus = analyzes[0].Status
		}
	} else {
		lastCompleteAnalysis = analyzes[0]
		if as.HasPendingChanges {
			// next analysis isn't running because of rate-limiting, but planned
			nextAnalysisStatus = "planned"
		}
	}

	lastCompleteAnalysis.RepoAnalysisStatus = as
	resp := response{
		RepoAnalysis:       lastCompleteAnalysis,
		GithubRepoName:     repoName,
		NextAnalysisStatus: nextAnalysisStatus,
	}

	ctx.ReturnJSON(resp)
	return nil
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/repoanalyzes/{analysisID}", handleRepoAnalysis)
	handlers.Register("/v1/repos/{owner}/{name}/repoanalyzes", handleRepoAnalyzesStatus)
}
