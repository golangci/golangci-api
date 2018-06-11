package repos

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
)

type State struct {
	Status              string
	ReportedIssuesCount int
	ResultJSON          json.RawMessage
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
	var analysis models.GithubAnalysis
	err := models.NewGithubAnalysisQuerySet(db.Get(&ctx)).
		GithubDeliveryGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get github analysis with guid %s", analysisGUID)
	}

	ctx.ReturnJSON(State{
		Status:              analysis.Status,
		ReportedIssuesCount: analysis.ReportedIssuesCount,
		ResultJSON:          analysis.ResultJSON,
	})
	return nil
}

func updateAnalysisState(ctx context.C) error {
	var payload State
	if err := json.NewDecoder(ctx.R.Body).Decode(&payload); err != nil {
		return herrors.New400Errorf("invalid payload json: %s", err)
	}

	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.GithubAnalysis
	err := models.NewGithubAnalysisQuerySet(db.Get(&ctx)).
		GithubDeliveryGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get github analysis with guid %s", analysisGUID)
	}

	prevStatus := analysis.Status
	analysis.Status = payload.Status
	analysis.ReportedIssuesCount = payload.ReportedIssuesCount
	analysis.ResultJSON = payload.ResultJSON
	err = analysis.Update(db.Get(&ctx),
		models.GithubAnalysisDBSchema.Status,
		models.GithubAnalysisDBSchema.ReportedIssuesCount,
		models.GithubAnalysisDBSchema.ResultJSON)
	if err != nil {
		return herrors.New(err, "can't update stats")
	}

	ctx.L.Infof("Updated analysis %s status: %s -> %s", analysisGUID, prevStatus, analysis.Status)
	return nil
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/analyzes/{analysisID}/status", handleAnalysisState)
	handlers.Register("/v1/repos/{owner}/{name}/analyzes/{analysisID}/state", handleAnalysisState)
}
