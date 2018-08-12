package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in repo_analysis_status.go

// gen:qs
type RepoAnalysisStatus struct {
	gorm.Model

	Name           string
	LastAnalyzedAt time.Time

	HasPendingChanges bool
	PendingCommitSHA  string
	Version           int
	DefaultBranch     string
}
