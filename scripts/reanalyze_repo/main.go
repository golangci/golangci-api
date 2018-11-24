package main

import (
	"flag"
	"log"

	"github.com/golangci/golangci-api/pkg/app"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue"
	"github.com/golangci/golangci-worker/app/analyze/analyzequeue/task"
	"github.com/jinzhu/gorm"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
)

func main() {
	repoName := flag.String("repo", "", "owner/name")
	flag.Parse()

	if *repoName == "" {
		log.Fatalf("Must set --repo")
	}

	if err := reanalyzeRepo(*repoName); err != nil {
		log.Fatalf("Failed to reanalyze: %s", err)
	}

	log.Printf("Successfully reanalyzed repo %s", *repoName)
}

func reanalyzeRepo(repoName string) error {
	a := app.NewApp()
	a.InitQueue()

	log := logutil.NewStderrLog("")
	cfg := config.NewEnvConfig(log)
	db, err := gormdb.GetDB(cfg, log, "")
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	var repo models.Repo
	if err = models.NewRepoQuerySet(db).NameEq(repoName).One(&repo); err != nil {
		return errors.Wrap(err, "failed to get repo by name")
	}

	return restartAnalysis(db, &repo)
}

func restartAnalysis(db *gorm.DB, repo *models.Repo) error {
	var as models.RepoAnalysisStatus
	if err := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(repo.ID).One(&as); err != nil {
		return errors.Wrapf(err, "can't get repo analysis status for repo %d", repo.ID)
	}

	var a models.RepoAnalysis
	if err := models.NewRepoAnalysisQuerySet(db).RepoAnalysisStatusIDEq(as.ID).OrderDescByID().One(&a); err != nil {
		return errors.Wrap(err, "can't get repo analysis")
	}

	t := &task.RepoAnalysis{
		Name:         repo.Name,
		AnalysisGUID: a.AnalysisGUID,
		Branch:       as.DefaultBranch,
	}

	if err := analyzequeue.ScheduleRepoAnalysis(t); err != nil {
		return errors.Wrapf(err, "can't resend repo %s for analysis into queue", repo.Name)
	}

	return nil
}
