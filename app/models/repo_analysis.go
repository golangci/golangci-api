package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in repo_analysis.go

// gen:qs
type RepoAnalysis struct {
	gorm.Model

	RepoAnalysisStatusID uint
	RepoAnalysisStatus   RepoAnalysisStatus

	AnalysisGUID string
	Status       string
	ResultJSON   []byte
}

func (RepoAnalysis) TableName() string {
	return "repo_analyzes"
}
