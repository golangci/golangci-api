package main

import (
	"flag"
	"log"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/repoanalyzesqueue"

	app "github.com/golangci/golangci-api/pkg/api"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/pkg/api/models"
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

	log := logutil.NewStderrLog("")
	cfg := config.NewEnvConfig(log)
	db, err := gormdb.GetDB(cfg, log, "")
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	var repo models.Repo
	if err = models.NewRepoQuerySet(db).FullNameEq(repoName).One(&repo); err != nil {
		return errors.Wrap(err, "failed to get repo by name")
	}

	return restartAnalysis(db, &repo, a.GetRepoAnalyzesRunQueue())
}

func restartAnalysis(db *gorm.DB, repo *models.Repo, runQueue *repoanalyzesqueue.Producer) error {
	var as models.RepoAnalysisStatus
	if err := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(repo.ID).One(&as); err != nil {
		return errors.Wrapf(err, "can't get repo analysis status for repo %d", repo.ID)
	}

	var a models.RepoAnalysis
	if err := models.NewRepoAnalysisQuerySet(db).RepoAnalysisStatusIDEq(as.ID).OrderDescByID().One(&a); err != nil {
		return errors.Wrap(err, "can't get repo analysis")
	}

	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(repo.UserID).One(&auth); err != nil {
		return errors.Wrap(err, "can't get auth")
	}

	return runQueue.Put(repo.FullName, a.AnalysisGUID, as.DefaultBranch, auth.PrivateAccessToken, a.CommitSHA)
}
