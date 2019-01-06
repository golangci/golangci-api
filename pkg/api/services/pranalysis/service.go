package pranalysis

import (
	"encoding/json"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
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
	//url:/v1/repos/{provider}/{owner}/{name}/analyzes/{analysisguid}/state
	GetAnalysisStateByAnalysisGUID(rc *request.InternalContext, req *AnalyzedRepo) (*State, error)

	//url:/v1/repos/{provider}/{owner}/{name}/pulls/{pullrequestnumber}
	GetAnalysisStateByPRNumber(rc *request.AnonymousContext, req *RepoPullRequest) (*State, error)

	//url:/v1/repos/{provider}/{owner}/{name}/analyzes/{analysisguid}/state method:PUT
	UpdateAnalysisStateByAnalysisGUID(rc *request.InternalContext, req *AnalyzedRepo, state *State) error
}

type BasicService struct {
	RepoPolicy *policy.Repo
}

func (s BasicService) GetAnalysisStateByAnalysisGUID(rc *request.InternalContext, req *AnalyzedRepo) (*State, error) {
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
		GithubRepoName:          repo.FullName,
	}, nil
}

func (s BasicService) GetAnalysisStateByPRNumber(rc *request.AnonymousContext, req *RepoPullRequest) (*State, error) {
	var repos []models.Repo // use could have reconnected repo so we would have two repos
	err := models.NewRepoQuerySet(rc.DB.Unscoped()).
		FullNameEq(req.FullName()).ProviderEq(req.Provider).
		OrderDescByCreatedAt().
		All(&repos)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get repo from db")
	}

	if len(repos) == 0 {
		return nil, errors.Wrapf(apierrors.ErrNotFound, "failed to find repos with name %s", req.FullName())
	}

	if repos[0].IsPrivate {
		if err = s.RepoPolicy.CanReadPrivateRepo(rc, &repos[0]); err != nil {
			return nil, err
		}
	}

	var repoIDs []uint
	for _, r := range repos {
		repoIDs = append(repoIDs, r.ID)
	}

	var analysis models.PullRequestAnalysis
	err = models.NewPullRequestAnalysisQuerySet(rc.DB).
		PullRequestNumberEq(req.PullRequestNumber).
		RepoIDIn(repoIDs...).
		OrderDescByID(). // get last
		Limit(1).
		One(&analysis)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get pull request analysis with number %d and repo ids %v",
			req.PullRequestNumber, repoIDs)
	}

	return &State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          repos[0].FullName,
	}, nil
}

func (s BasicService) UpdateAnalysisStateByAnalysisGUID(rc *request.InternalContext, req *AnalyzedRepo, state *State) error {
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
