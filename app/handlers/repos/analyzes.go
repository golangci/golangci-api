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

type statusPayload struct {
	Status string
}

func handleAnalysisStatus(ctx context.C) error {
	switch ctx.R.Method {
	case http.MethodGet:
		return getAnalysisStatus(ctx)
	case http.MethodPut:
		return updateAnalysisStatus(ctx)
	default:
		return fmt.Errorf("not allowed method")
	}
}

func getAnalysisStatus(ctx context.C) error {
	analysisGUID := ctx.URLVar("analysisID")
	var analysis models.GithubAnalysis
	err := models.NewGithubAnalysisQuerySet(db.Get(&ctx)).
		GithubDeliveryGUIDEq(analysisGUID).
		One(&analysis)
	if err != nil {
		return herrors.New(err, "can't get github analysis with guid %s", analysisGUID)
	}

	ctx.ReturnJSON(statusPayload{
		Status: analysis.Status,
	})
	return nil
}

func updateAnalysisStatus(ctx context.C) error {
	var payload statusPayload
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
	if err = analysis.Update(db.Get(&ctx), models.GithubAnalysisDBSchema.Status); err != nil {
		return herrors.New(err, "can't update stats")
	}

	ctx.L.Infof("Updated analysis %s status: %s -> %s", analysisGUID, prevStatus, analysis.Status)
	return nil
}

func init() {
	handlers.Register("/v1/repos/{owner}/{name}/analyzes/{analysisID}/status", handleAnalysisStatus)
}
