package main

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

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
	if err := buildUsersList(); err != nil {
		log.Fatalf("Failed to build users list: %s", err)
	}
	log.Printf("Successfully build users list")
}

func buildUsersList() error {
	log := logutil.NewStderrLog("")
	cfg := config.NewEnvConfig(log)
	db, err := gormdb.GetDB(cfg, log, "")
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	var users []models.User
	if err = models.NewUserQuerySet(db).All(&users); err != nil {
		return errors.Wrap(err, "failed to get users")
	}

	lines := []string{"email,"}
	seenEmails := map[string]bool{}
	for _, u := range users {
		email := strings.ToLower(u.Email)
		if !strings.Contains(email, "@") {
			continue
		}

		if seenEmails[email] {
			continue
		}
		seenEmails[email] = true

		lines = append(lines, email)
	}

	if err = ioutil.WriteFile("users.csv", []byte(strings.Join(lines, "\n")), os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to write result to file")
	}

	return nil
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
