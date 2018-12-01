package repostate

import (
	"context"
	"time"
)

//go:generate mockgen -package repostate -source storage.go -destination storage_mock.go

type State struct {
	CreatedAt  time.Time
	Status     string
	ResultJSON interface{}
}

type Storage interface {
	UpdateState(ctx context.Context, owner, name, analysisID string, state *State) error
	GetState(ctx context.Context, owner, name, analysisID string) (*State, error)
}
