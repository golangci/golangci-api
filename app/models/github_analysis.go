package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in github_analysis.go

// gen:qs
type GithubAnalysis struct {
	gorm.Model

	GithubRepo              GithubRepo
	GithubRepoID            uint
	GithubPullRequestNumber int
	GithubDeliveryGUID      string

	Status              string
	ReportedIssuesCount int
}

func (GithubAnalysis) TableName() string {
	return "github_analyzes"
}
