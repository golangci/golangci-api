package models

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in repo_analysis.go

// gen:qs
type RepoAnalysis struct {
	gorm.Model

	RepoAnalysisStatusID uint
	RepoAnalysisStatus   RepoAnalysisStatus

	AnalysisGUID   string
	Status         string
	CommitSHA      string
	ResultJSON     json.RawMessage
	AttemptNumber  int
	LintersVersion string
}

func (RepoAnalysis) TableName() string {
	return "repo_analyzes"
}
