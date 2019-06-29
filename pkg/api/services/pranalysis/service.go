package pranalysis

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/pkg/api/policy"

	"github.com/golangci/golangci-api/internal/api/apierrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
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

	PreviousAnalyzes []SamePullStateLink `json:",omitempty"`
}

type SamePullStateLink struct {
	CommitSHA string
	CreatedAt time.Time
}

func stateFromAnalysis(analysis *models.PullRequestAnalysis, fullName string) *State {
	return &State{
		Model:                   analysis.Model,
		Status:                  analysis.Status,
		ReportedIssuesCount:     analysis.ReportedIssuesCount,
		ResultJSON:              analysis.ResultJSON,
		CommitSHA:               analysis.CommitSHA,
		GithubPullRequestNumber: analysis.PullRequestNumber,
		GithubRepoName:          fullName,
	}
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
	PullRequestNumber int    `request:",urlPart,"`
	CommitSHA         string `request:"commit_sha,urlParam,optional"`
}

func (r RepoPullRequest) FillLogContext(lctx logutil.Context) {
	r.Repo.FillLogContext(lctx)
	if r.CommitSHA != "" {
		lctx["commit_sha"] = r.CommitSHA
	}
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
	Pf         providers.Factory
	Cfg        config.Config
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

	return stateFromAnalysis(&analysis, repo.FullName), nil
}

func (s BasicService) tryGetRenamedRepo(rc *request.AnonymousContext, req *RepoPullRequest, repos *[]models.Repo) error {
	configKey := strings.ToUpper(fmt.Sprintf("%s_SERVICE_ACCESS_TOKEN", strings.Replace(req.Provider, ".", "_", -1)))
	token := s.Cfg.GetString(configKey)
	if token == "" {
		return fmt.Errorf("no %s config param", configKey)
	}

	p, err := s.Pf.BuildForToken(req.Provider, token)
	if err != nil {
		return errors.Wrap(err, "failed to build provider for service user")
	}

	providerRepo, err := p.GetRepoByName(rc.Ctx, req.Owner, req.Name)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch repo %s from provider", req.FullNameWithProvider())
	}

	if err = models.NewRepoQuerySet(rc.DB).ProviderEq(req.Provider).ProviderIDEq(providerRepo.ID).All(repos); err != nil {
		return errors.Wrapf(err, "failed to fetch repo with provider id %d (%s)",
			providerRepo.ID, req.FullNameWithProvider())
	}

	if len(*repos) == 0 {
		return fmt.Errorf("repos with provider id %d weren't found", providerRepo.ID)
	}

	rc.Log.Infof("Fetched and used renamed provider repo %#v by name %s", providerRepo, req.FullNameWithProvider())
	return nil
}

func (s BasicService) getRepoIDsForPullRequest(rc *request.AnonymousContext, req *RepoPullRequest) ([]uint, string, error) {
	var repos []models.Repo // use could have reconnected repo so we would have two repos
	err := models.NewRepoQuerySet(rc.DB.Unscoped()).
		FullNameEq(req.FullName()).ProviderEq(req.Provider).
		OrderDescByCreatedAt().
		All(&repos)
	if err != nil {
		return nil, "", errors.Wrapf(err, "can't get repo from db")
	}

	if len(repos) == 0 {
		if tryErr := s.tryGetRenamedRepo(rc, req, &repos); tryErr != nil {
			rc.Log.Warnf("Failed to check renamed repo: %s", tryErr)
			return nil, "", errors.Wrapf(apierrors.ErrNotFound, "failed to find repos with name %s", req.FullName())
		}

		// continue, found renamed repo
	}

	if repos[0].IsPrivate {
		if err = s.RepoPolicy.CanReadPrivateRepo(rc, &repos[0]); err != nil {
			return nil, "", err
		}
	}

	var repoIDs []uint
	for _, r := range repos {
		repoIDs = append(repoIDs, r.ID)
	}

	return repoIDs, repos[0].FullName, nil
}

func (s BasicService) GetAnalysisStateByPRNumber(rc *request.AnonymousContext, req *RepoPullRequest) (*State, error) {
	repoIDs, fullName, err := s.getRepoIDsForPullRequest(rc, req)
	if err != nil {
		return nil, err
	}

	qs := models.NewPullRequestAnalysisQuerySet(rc.DB).
		PullRequestNumberEq(req.PullRequestNumber).
		RepoIDIn(repoIDs...).
		OrderDescByID()
	if req.CommitSHA != "" {
		var analysis models.PullRequestAnalysis
		if err = qs.CommitSHAEq(req.CommitSHA).One(&analysis); err != nil {
			return nil, errors.Wrapf(err, "can't get pull request analysus with number %d and repo ids %v and commit sha %s",
				req.PullRequestNumber, repoIDs, req.CommitSHA)
		}
		return stateFromAnalysis(&analysis, fullName), nil
	}

	const maxPreviousAnalyzesCount = 5

	var analyzes []models.PullRequestAnalysis
	err = qs.Limit(maxPreviousAnalyzesCount * 3 /* reserve for repeating commitSHA */).All(&analyzes)
	if err != nil {
		return nil, errors.Wrapf(err, "can't get pull request analyzes with number %d and repo ids %v",
			req.PullRequestNumber, repoIDs)
	}
	if len(analyzes) == 0 {
		return nil, fmt.Errorf("got 0 pull request analyzes for repo ids %v", repoIDs)
	}

	state := stateFromAnalysis(&analyzes[0], fullName)

	seenCommitSHAs := map[string]bool{state.CommitSHA: true}
	for _, a := range analyzes[1:] {
		if seenCommitSHAs[a.CommitSHA] {
			// TODO: make uniq index and remove it
			continue
		}

		seenCommitSHAs[a.CommitSHA] = true
		state.PreviousAnalyzes = append(state.PreviousAnalyzes, SamePullStateLink{CommitSHA: a.CommitSHA, CreatedAt: a.CreatedAt})
		if len(state.PreviousAnalyzes) == maxPreviousAnalyzesCount {
			break
		}
	}
	return state, nil
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
