package repos

import (
	"encoding/json"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/helpers"
	"github.com/golangci/golib/server/handlers/herrors"
)

type statusUpdatePayload struct {
	Status string
}

func updateAnalysisStatus(ctx context.C) error {
	var payload statusUpdatePayload
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
	handlers.Register("/v1/repos/{owner}/{name}/analyzes/{analysisID}/status", helpers.OnlyPUT(updateAnalysisStatus))
}
