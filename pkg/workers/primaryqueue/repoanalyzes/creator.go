package repoanalyzes

import (
	"context"
	"fmt"

	"github.com/golangci/golangci-api/pkg/analyzes/repoanalyzes"
	"github.com/golangci/golangci-api/pkg/queue/producers"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

const createQeuueID = "repoanalyzes/create"

type createMessage struct {
	RepoID uint
}

func (m createMessage) DeduplicationID() string {
	return fmt.Sprintf("%s/%d", createQeuueID, m.RepoID)
}

type CreatorProducer struct {
	q producers.Queue
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	q, err := m.NewSubqueue(createQeuueID)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s subqueue", createQeuueID)
	}

	cp.q = q
	return nil
}

func (cp CreatorProducer) Put(repoID uint) error {
	return cp.q.Put(createMessage{
		RepoID: repoID,
	})
}

type CreatorConsumer struct {
	log     logutil.Log
	fetcher *repoanalyzes.GithubRepoStateFetcher
	db      *gorm.DB
}

func NewCreatorConsumer(log logutil.Log, db *gorm.DB, fetcher *repoanalyzes.GithubRepoStateFetcher) *CreatorConsumer {
	return &CreatorConsumer{
		log:     log,
		db:      db,
		fetcher: fetcher,
	}
}

func (cc CreatorConsumer) Register(m *consumers.Multiplexer) error {
	consumer, err := consumers.NewReflectConsumer(cc.consumeMessage)
	if err != nil {
		return errors.Wrap(err, "can't register launch handler")
	}

	return m.RegisterConsumer(createQeuueID, consumer)
}

func (cc CreatorConsumer) consumeMessage(ctx context.Context, m *createMessage) error {
	var repo models.Repo
	if err := models.NewRepoQuerySet(cc.db).IDEq(m.RepoID).One(&repo); err != nil {
		return errors.Wrapf(err, "failed to find repo with id %d", m.RepoID)
	}

	active := true
	state, err := cc.fetcher.Fetch(ctx, &repo)
	if err != nil {
		active = false
		cc.log.Warnf("Create analysis for the new repo: mark repo as inactive: "+
			"can't fetch initial state for repo %s: %s", repo.Name, err)
		state = &repoanalyzes.GithubRepoState{}
	}

	as := models.RepoAnalysisStatus{
		DefaultBranch:     state.DefaultBranch,
		PendingCommitSHA:  state.HeadCommitSHA,
		HasPendingChanges: true,
		Active:            active,
		RepoID:            repo.ID,
	}
	if err = as.Create(cc.db); err != nil {
		return errors.Wrap(err, "can't create analysis status in db")
	}

	cc.log.Infof("Created new repo analysis status: %#v", as)
	return nil
}
