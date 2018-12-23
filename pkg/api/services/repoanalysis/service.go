package repoanalysis

import (
	"strings"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type Status struct {
	models.RepoAnalysis
	GithubRepoName     string
	NextAnalysisStatus string `json:",omitempty"`
	IsPreparing        bool   `json:",omitempty"`
	RepoIsNotConnected bool   `json:",omitempty"`
}

type Context struct {
	request.Repo
	AnalysisGUID string `request:",urlPart,"`
}

func (c Context) FillLogContext(lctx logutil.Context) {
	lctx["analysis_guid"] = c.AnalysisGUID
	c.Repo.FillLogContext(lctx)
}

type updateRepoPayload models.RepoAnalysis

func (p updateRepoPayload) FillLogContext(lctx logutil.Context) {}

type Service interface {
	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes
	GetStatus(rc *request.AnonymousContext, repo *request.Repo) (*Status, error)

	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes/{analysisguid}
	GetByAnalysisGUID(rc *request.InternalContext, rac *Context) (*models.RepoAnalysis, error)

	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes/{analysisguid} method:PUT
	UpdateByAnalysisGUID(rc *request.InternalContext, rac *Context, update *updateRepoPayload) error
}

type BasicService struct {
	RepoPolicy *policy.Repo
}

func (s BasicService) isCompleteAnalysisStatus(status string) bool {
	return status == "processed" || status == "error"
}

//nolint:gocyclo
func (s BasicService) GetStatus(rc *request.AnonymousContext, reqRepo *request.Repo) (*Status, error) {
	var repo models.Repo
	err := models.NewRepoQuerySet(rc.DB).FullNameEq(strings.ToLower(reqRepo.FullName())).One(&repo)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			rc.Log.Warnf("no connected repo for report of %s: maybe direct access by URL", reqRepo.FullName())
			return &Status{
				RepoIsNotConnected: true,
				GithubRepoName:     reqRepo.FullName(),
			}, nil
		}

		return nil, errors.Wrapf(err, "can't get repo for %s", reqRepo.FullName())
	}

	if repo.IsPrivate {
		if err = s.RepoPolicy.CanReadPrivateRepo(rc, &repo); err != nil {
			return nil, err
		}
	}

	var as models.RepoAnalysisStatus
	err = models.NewRepoAnalysisStatusQuerySet(rc.DB).RepoIDEq(repo.ID).One(&as)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &Status{
				IsPreparing:    true,
				GithubRepoName: repo.DisplayFullName,
			}, nil
		}

		return nil, errors.Wrapf(err, "can't get repo analysis status for %s and repo id %d",
			reqRepo.FullName(), repo.ID)
	}

	var analyzes []models.RepoAnalysis
	err = models.NewRepoAnalysisQuerySet(rc.DB).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID(). // get last
		Limit(2).
		All(&analyzes)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo analyzes with analysis status id %d", as.ID)
	}

	return s.buildStatus(analyzes, &repo, &as), nil
}

func (s BasicService) buildStatus(analyzes []models.RepoAnalysis, repo *models.Repo, as *models.RepoAnalysisStatus) *Status {
	if len(analyzes) == 0 {
		return &Status{
			IsPreparing:    true,
			GithubRepoName: repo.DisplayFullName,
		}
	}

	var lastCompleteAnalysis models.RepoAnalysis
	var nextAnalysisStatus string

	if !s.isCompleteAnalysisStatus(analyzes[0].Status) { // the last analysis is running now
		if len(analyzes) == 1 || !s.isCompleteAnalysisStatus(analyzes[1].Status) {
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

	lastCompleteAnalysis.RepoAnalysisStatus = *as
	return &Status{
		RepoAnalysis:       lastCompleteAnalysis,
		GithubRepoName:     repo.DisplayFullName,
		NextAnalysisStatus: nextAnalysisStatus,
	}
}

func (s BasicService) GetByAnalysisGUID(rc *request.InternalContext, rac *Context) (*models.RepoAnalysis, error) {
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(rc.DB).
		AnalysisGUIDEq(rac.AnalysisGUID).
		One(&analysis)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo analysis with guid %s", rac.AnalysisGUID)
	}

	return &analysis, nil
}

func (s BasicService) UpdateByAnalysisGUID(rc *request.InternalContext, rac *Context, update *updateRepoPayload) error {
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(rc.DB).
		AnalysisGUIDEq(rac.AnalysisGUID).
		One(&analysis)
	if err != nil {
		return errors.Wrapf(err, "can't get repo analysis with guid %s", rac.AnalysisGUID)
	}

	prevStatus := analysis.Status
	analysis.Status = update.Status
	analysis.ResultJSON = update.ResultJSON
	if analysis.ResultJSON == nil {
		analysis.ResultJSON = []byte("{}")
	}
	err = analysis.Update(rc.DB,
		models.RepoAnalysisDBSchema.Status,
		models.RepoAnalysisDBSchema.ResultJSON)
	if err != nil {
		return errors.Wrap(err, "can't update repo analysis")
	}

	rc.Log.Infof("Updated repo analysis %s state: status: %s -> %s", rac.AnalysisGUID, prevStatus, analysis.Status)
	return nil
}
