package pranalysis

import (
	"encoding/json"

	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/request"
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

func (s State) FillLogContext(lctx logutil.Context) {
	lctx["status"] = s.Status
	lctx["reported_issues"] = s.ReportedIssuesCount
}

type AnalyzedRepo struct {
	request.Repo
	AnalysisGUID string `request:",urlPart,"`
}

type RepoPullRequest struct {
	request.Repo
	PullRequestNumber int `request:",urlPart,"`
}

type Service interface {
	//url:/v1/repos/{provider}/{owner}/{name}/analyzes/{analysisguid}/state method:GET
	GetAnalysisStateByAnalysisGUID(rc *request.AnonymousContext, req *AnalyzedRepo) (*State, error)

	//url:/v1/repos/{provider}/{owner}/{name}/pulls/{pullrequestnumber} method:GET
	GetAnalysisStateByPRNumber(rc *request.AnonymousContext, req *RepoPullRequest) (*State, error)

	//url:/v1/repos/{provider}/{owner}/{name}/analyzes/{analysisguid}/state method:PUT
	UpdateAnalysisStateByAnalysisGUID(rc *request.AnonymousContext, req *AnalyzedRepo, state *State) error
}

type BasicService struct{}

func (s BasicService) GetAnalysisStateByAnalysisGUID(rc *request.AnonymousContext, req *AnalyzedRepo) (*State, error) {
	var analysis models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(rc.DB).GithubDeliveryGUIDEq(req.AnalysisGUID).One(&analysis)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get analysis with guid %s", req.AnalysisGUID)
	}

	var repo models.Repo
	err = models.NewRepoQuerySet(rc.DB).IDEq(analysis.RepoID).One(&repo)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo id %d", analysis.RepoID)
	}

	return &State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          repo.Name,
	}, nil
}

func (s BasicService) GetAnalysisStateByPRNumber(rc *request.AnonymousContext, req *RepoPullRequest) (*State, error) {
	var repo models.Repo
	err := models.NewRepoQuerySet(rc.DB).NameEq(req.FullName()).ProviderEq(req.Provider).One(&repo)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo from db")
	}

	var analysis models.PullRequestAnalysis
	err = models.NewPullRequestAnalysisQuerySet(rc.DB).
		PullRequestNumberEq(req.PullRequestNumber).
		RepoIDEq(repo.ID).
		OrderDescByID(). // get last
		Limit(1).
		One(&analysis)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get pull request analysis with number %d and repo id %d",
			req.PullRequestNumber, repo.ID)
	}

	return &State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          repo.Name,
	}, nil
}

func (s BasicService) UpdateAnalysisStateByAnalysisGUID(rc *request.AnonymousContext, req *AnalyzedRepo, state *State) error {
	var analysis models.PullRequestAnalysis
	err := models.NewPullRequestAnalysisQuerySet(rc.DB).GithubDeliveryGUIDEq(req.AnalysisGUID).One(&analysis)
	if err != nil {
		return errors.Wrapf(err, "can't get pull request analysis with guid %s", req.AnalysisGUID)
	}

	prevStatus := analysis.Status
	analysis.Status = state.Status
	analysis.ReportedIssuesCount = state.ReportedIssuesCount
	analysis.ResultJSON = state.ResultJSON
	if analysis.ResultJSON == nil {
		analysis.ResultJSON = []byte("{}")
	}

	err = analysis.Update(rc.DB,
		models.PullRequestAnalysisDBSchema.Status,
		models.PullRequestAnalysisDBSchema.ReportedIssuesCount,
		models.PullRequestAnalysisDBSchema.ResultJSON)
	if err != nil {
		return errors.Wrapf(err, "can't update pr analysis state for analytis %#v", analysis)
	}

	rc.Log.Infof("Updated analysis %s status: %s -> %s", req.AnalysisGUID, prevStatus, analysis.Status)
	return nil
}
