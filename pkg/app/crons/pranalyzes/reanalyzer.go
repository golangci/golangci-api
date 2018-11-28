package pranalyzes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/app/providers"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/golangci/golangci-worker/app/lib/github"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"github.com/golangci/golangci-lint/pkg/printers"
)

type Reanalyzer struct {
	db  *gorm.DB
	cfg config.Config
	log logutil.Log
	pf  providers.Factory
}

func NewReanalyzer(db *gorm.DB, cfg config.Config, log logutil.Log, pf providers.Factory) *Reanalyzer {
	return &Reanalyzer{
		db:  db,
		cfg: cfg,
		log: log,
		pf:  pf,
	}
}

type state struct {
	ReanalyzedPullRequests map[string]bool
}

func (r Reanalyzer) dumpState(s *state) error {
	f, err := os.Create("state.json")
	if err != nil {
		return errors.Wrap(err, "failed to open state.json")
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(s); err != nil {
		return errors.Wrap(err, "failed to json marshal")
	}

	return nil
}

func (r Reanalyzer) loadState(s *state) error {
	f, err := os.Open("state.json")
	if err != nil {
		return errors.Wrap(err, "failed to open state.json")
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(s); err != nil {
		return errors.Wrap(err, "failed to json unmarshal")
	}

	return nil
}

func (r Reanalyzer) mergeAnalyzes(analyzes []models.PullRequestAnalysis) []models.PullRequestAnalysis {
	var ret []models.PullRequestAnalysis
	seen := map[string]bool{}
	for _, a := range analyzes {
		key := fmt.Sprintf("repoID=%d prNumber=%d", a.RepoID, a.PullRequestNumber)
		if seen[key] {
			continue
		}
		seen[key] = true
		ret = append(ret, a)
	}

	r.log.Infof("Combined %d analyzes to %d uniq-pr analyzes", len(analyzes), len(ret))
	return ret
}

func (r Reanalyzer) RunOnce() error {
	s := state{
		ReanalyzedPullRequests: map[string]bool{},
	}
	_ = r.loadState(&s)

	startedAt := time.Now()
	dur := r.cfg.GetDuration("PR_REANALYZER_DURATION", time.Hour*24*9)
	var analyzes []models.PullRequestAnalysis
	qs := models.NewPullRequestAnalysisQuerySet(r.db).CreatedAtGte(time.Now().Add(-dur)).OrderDescByID()
	if repoID := r.cfg.GetInt("PR_REANALYZER_REPO_ID_FILTER", 0); repoID != 0 {
		qs = qs.RepoIDEq(uint(repoID))
	}

	err := qs.All(&analyzes)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch last pr analyzes for %s", dur)
	}

	analyzes = r.mergeAnalyzes(analyzes)

	statusesDistribution := map[string]int{}
	for _, a := range analyzes {
		statusesDistribution[a.Status]++
	}

	r.log.Infof("Fetched %d pull request analyzes to check for recovery for last %s with statuses: %#v",
		len(analyzes), dur, statusesDistribution)

	defer func() {
		if err = r.dumpState(&s); err != nil {
			r.log.Warnf("Failed to dump state: %s", err)
		}
	}()

	for i, a := range analyzes {
		ctx := context.Background()
		if err = r.processAnalysis(ctx, &a, i, &s); err != nil {
			r.log.Warnf("Failed to process analysis %d: %s", a.ID, err)
		}
	}

	r.log.Infof("Finished reanalyzing for %s", time.Since(startedAt))
	return nil
}

type resultJSON struct {
	Version         int
	GolangciLintRes printers.JSONResult
	WorkerRes       workerRes
}

type workerRes struct {
	Timings  []Timing  `json:",omitempty"`
	Warnings []Warning `json:",omitempty"`
	Error    string    `json:",omitempty"`
}

type JSONDuration time.Duration

type Timing struct {
	Name     string
	Duration JSONDuration `json:"DurationMs"`
}

type Warning struct {
	Tag  string
	Text string
}

//nolint:gocyclo
func (r *Reanalyzer) processAnalysis(ctx context.Context, a *models.PullRequestAnalysis, i int, s *state) error {
	var repo models.Repo
	if err := models.NewRepoQuerySet(r.db.Unscoped()).IDEq(a.RepoID).One(&repo); err != nil {
		return errors.Wrapf(err, "failed to fetch repo %d", a.RepoID)
	}

	p, err := r.pf.BuildForUser(r.db, repo.UserID)
	if err != nil {
		return errors.Wrapf(err, "failed to build provider for user %d", repo.UserID)
	}

	prLink := p.LinkToPullRequest(&repo, a.PullRequestNumber)
	if s.ReanalyzedPullRequests[prLink] {
		return nil
	}

	link := fmt.Sprintf("%s ID=%d", prLink, a.ID)
	if repo.DeletedAt != nil {
		r.log.Warnf("#%d: %s repo was disconnected, skip (%s ago)", i, link, time.Since(a.CreatedAt))
		return nil
	}

	var res resultJSON
	if err = json.Unmarshal(a.ResultJSON, &res); err != nil {
		return errors.Wrapf(err, "invalid result json")
	}

	var reason string
	if a.Status != "processed/success" && a.Status != "processed/failure" {
		reason = "not success status"
	} else if len(res.WorkerRes.Warnings) != 0 {
		reason = fmt.Sprintf("warnings: %v", res.WorkerRes.Warnings)
	} else if res.GolangciLintRes.Report != nil && res.GolangciLintRes.Report.Error != "" {
		reason = "golangci-lint error: " + res.GolangciLintRes.Report.Error
	} else if res.GolangciLintRes.Report != nil && len(res.GolangciLintRes.Report.Warnings) != 0 {
		gw := res.GolangciLintRes.Report.Warnings
		if !(len(gw) == 1 &&
			strings.Contains(gw[0].Text, "Can't run megacheck because of compilation errors")) {

			reason = fmt.Sprintf("golangci-lint warnings: %v", gw)
		}
	}
	if reason == "" {
		return nil
	}

	pr, err := p.GetPullRequest(ctx, repo.Owner(), repo.Repo(), a.PullRequestNumber)
	if err != nil {
		return errors.Wrapf(err, "failed to get pull request %s", link)
	}

	if pr.State == "merged" || pr.State == "closed" {
		r.log.Warnf("#%d: %s is already %s, can't reanalyze (%s ago)",
			i, link, pr.State, time.Since(a.CreatedAt))
		s.ReanalyzedPullRequests[prLink] = true
		return nil
	}

	r.log.Infof("#%d: %s in state %s with status %s is starting reanalyzing (%s ago): reason is %s",
		i, link, pr.State, a.Status, time.Since(a.CreatedAt), reason)

	if err = r.restartAnalysis(a, &repo); err != nil {
		return errors.Wrapf(err, "failed to restart pull request %s analysis", link)
	}

	startedPollingAt := time.Now()
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	for range t.C {
		var updatedAnalysis models.PullRequestAnalysis
		err = models.NewPullRequestAnalysisQuerySet(r.db).IDEq(a.ID).One(&updatedAnalysis)
		if err == nil && updatedAnalysis.UpdatedAt.After(a.UpdatedAt) {
			r.log.Infof("%s was reanalyzed for %s, status: %s -> %s",
				link, time.Since(startedPollingAt), a.Status, updatedAnalysis.Status)
			break
		}
		r.log.Infof("Polling: err: %s, updated at: new=%s, prev=%s", err, updatedAnalysis.UpdatedAt, a.UpdatedAt)

		if time.Since(startedPollingAt) > 3*time.Minute {
			r.log.Warnf("Waiting too long for %s to finish, proceed", link)
			break
		}
	}

	s.ReanalyzedPullRequests[prLink] = true
	return nil
}

func (r Reanalyzer) restartAnalysis(a *models.PullRequestAnalysis, repo *models.Repo) error {
	var auth models.Auth
	if err := models.NewAuthQuerySet(r.db).UserIDEq(repo.UserID).One(&auth); err != nil {
		return errors.Wrapf(err, "failed to get auth for repo %d", repo.ID)
	}

	githubCtx := github.Context{
		Repo: github.Repo{
			Owner: repo.Owner(),
			Name:  repo.Repo(),
		},
		GithubAccessToken: auth.StrongestAccessToken(), // TODO: get strongest only if paid
		PullRequestNumber: a.PullRequestNumber,
	}
	t := &task.PRAnalysis{
		Context:      githubCtx,
		UserID:       repo.UserID,
		AnalysisGUID: a.GithubDeliveryGUID,
	}

	if err := analyzequeue.SchedulePRAnalysis(t); err != nil {
		return errors.Wrap(err, "can't send pull request for analysis into queue: %s")
	}

	return nil
}
