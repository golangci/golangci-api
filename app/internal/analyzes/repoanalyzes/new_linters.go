package repoanalyzes

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/app/internal/db"
	"github.com/golangci/golangci-api/app/internal/errors"
	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/golangci/golib/server/context"
	"github.com/jinzhu/gorm"
)

func reanalyzeByNewLinters() {
	ctx := utils.NewBackgroundContext()

	checkInterval := time.Minute * 2
	for range time.Tick(checkInterval) {
		if err := reanalyzeByNewLintersIter(ctx); err != nil {
			errors.Warnf(ctx, "Can't reanalyze by new linters: %s", err)
			continue
		}
	}
}

func reanalyzeByNewLintersIter(ctx *context.C) error {
	var a models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(db.Get(ctx)).
		LintersVersionNe(lintersVersion).
		One(&a)
	if err == gorm.ErrRecordNotFound {
		return nil // nothing to reanalyze
	}

	var as models.RepoAnalysisStatus
	err = models.NewRepoAnalysisStatusQuerySet(db.Get(ctx)).IDEq(a.RepoAnalysisStatusID).One(&as)
	if err != nil {
		return fmt.Errorf("can't get repo analysis by id %d: %s", a.RepoAnalysisStatusID, err)
	}

	as.HasPendingChanges = true
	as.PendingCommitSHA = a.CommitSHA
	err = as.Update(db.Get(ctx),
		models.RepoAnalysisStatusDBSchema.HasPendingChanges,
		models.RepoAnalysisStatusDBSchema.PendingCommitSHA,
	)
	if err != nil {
		return fmt.Errorf("can't update has_pending_changes to true: %s", err)
	}

	ctx.L.Infof("Mark repo %s for reanalysis by new linters", as.Name)
	return nil
}
