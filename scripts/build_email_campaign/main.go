package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/pkg/api/models"
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

	fmt.Println(strings.Join(lines, "\n"))
	return nil
}
