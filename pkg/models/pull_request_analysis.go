package models

import (
	"github.com/jinzhu/gorm"
)

//go:generate goqueryset -in pull_request_analysis.go

//gen:qs
type PullRequestAnalysis struct {
	gorm.Model

	RepoID            uint
	PullRequestNumber int

	GithubDeliveryGUID string

	CommitSHA string

	Status              string
	ReportedIssuesCount int

	ResultJSON []byte
}

func (PullRequestAnalysis) TableName() string {
	return "pull_request_analyzes"
}
