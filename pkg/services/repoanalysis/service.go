package repoanalysis

import (
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-shared/pkg/logutil"
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
	AnalysisGUID string `request:",url"`
}

func (c Context) FillLogContext(lctx logutil.Context) {
	lctx["analysis_guid"] = c.AnalysisGUID
	c.Repo.FillLogContext(lctx)
}

type updateRepoPayload models.RepoAnalysis

func (p updateRepoPayload) FillLogContext(lctx logutil.Context) {}

type Service interface {
	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes
	GetStatus(rc *request.Context, repo *request.Repo) (*Status, error)

	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes/{analysisguid}
	Get(rc *request.Context, rac *Context) (*models.RepoAnalysis, error)

	//url:/v1/repos/{provider}/{owner}/{name}/repoanalyzes/{analysisguid} method:PUT
	Update(rc *request.Context, rac *Context, update *updateRepoPayload) error
}

type BasicService struct {
	DB *gorm.DB
}

func (s BasicService) isCompleteAnalysisStatus(status string) bool {
	return status == "processed" || status == "error"
}

func (s BasicService) GetStatus(rc *request.Context, repo *request.Repo) (*Status, error) {
	repoName := repo.FullName()

	var as models.RepoAnalysisStatus
	err := models.NewRepoAnalysisStatusQuerySet(s.DB).NameEq(repoName).One(&as)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			n, _ := models.NewRepoQuerySet(s.DB).NameEq(repoName).Count()
			if n != 0 {
				return &Status{
					IsPreparing:    true,
					GithubRepoName: repoName,
				}, nil
			}

			rc.Log.Warnf("no connected repo for report of %s: maybe direct access by URL", repoName)
			return &Status{
				RepoIsNotConnected: true,
				GithubRepoName:     repoName,
			}, nil
		}

		return nil, errors.Wrapf(err, "can't get repo analysis status for %s", repoName)
	}

	var analyzes []models.RepoAnalysis
	err = models.NewRepoAnalysisQuerySet(s.DB).
		RepoAnalysisStatusIDEq(as.ID).
		OrderDescByID(). // get last
		Limit(2).
		All(&analyzes)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo analyzes with analysis status id %d", as.ID)
	}

	if len(analyzes) == 0 {
		return &Status{
			IsPreparing:    true,
			GithubRepoName: repoName,
		}, nil
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

	lastCompleteAnalysis.RepoAnalysisStatus = as
	return &Status{
		RepoAnalysis:       lastCompleteAnalysis,
		GithubRepoName:     repoName,
		NextAnalysisStatus: nextAnalysisStatus,
	}, nil
}

func (s BasicService) Get(rc *request.Context, rac *Context) (*models.RepoAnalysis, error) {
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(s.DB).
		AnalysisGUIDEq(rac.AnalysisGUID).
		One(&analysis)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo analysis with guid %s", rac.AnalysisGUID)
	}

	return &analysis, nil
}

func (s BasicService) Update(rc *request.Context, rac *Context, update *updateRepoPayload) error {
	var analysis models.RepoAnalysis
	err := models.NewRepoAnalysisQuerySet(s.DB).
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
	err = analysis.Update(s.DB,
		models.RepoAnalysisDBSchema.Status,
		models.RepoAnalysisDBSchema.ResultJSON)
	if err != nil {
		return errors.Wrap(err, "can't update repo analysis")
	}

	rc.Log.Infof("Updated repo analysis %s state: status: %s -> %s", rac.AnalysisGUID, prevStatus, analysis.Status)
	return nil
}
