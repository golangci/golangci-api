package repos

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/app/models"
	"github.com/golangci/golangci-api/pkg/app/providers"
	"github.com/golangci/golangci-api/pkg/app/providers/provider"
	"github.com/golangci/golangci-api/pkg/app/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-api/pkg/queue/producers"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"gopkg.in/redsync.v1"
)

const deleteQueueID = "repos/delete"

type deleteMessage struct {
	UserID uint
	RepoID uint
}

func (m deleteMessage) LockID() string {
	return fmt.Sprintf("%s/repo:%d/user:%d", deleteQueueID, m.RepoID, m.UserID)
}

type DeleterProducer struct {
	producers.Base
}

func (dp *DeleterProducer) Register(m *producers.Multiplexer) error {
	return dp.Base.Register(m, deleteQueueID)
}

func (dp DeleterProducer) Put(repoID, userID uint) error {
	return dp.Base.Put(deleteMessage{
		RepoID: repoID,
		UserID: userID,
	})
}

type DeleterConsumer struct {
	log             logutil.Log
	db              *sql.DB
	cfg             config.Config
	providerFactory providers.Factory
}

func NewDeleterConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pf providers.Factory) *DeleterConsumer {
	return &DeleterConsumer{
		log:             log,
		db:              db,
		cfg:             cfg,
		providerFactory: pf,
	}
}

func (dc DeleterConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(dc.consumeMessage, deleteQueueID, m, df)
}

func (dc DeleterConsumer) run(ctx context.Context, m *deleteMessage, gormDB *gorm.DB) error {
	// TODO: take provider error into account and delete repo with saving error description

	var repo models.Repo
	if err := models.NewRepoQuerySet(gormDB.Unscoped()).IDEq(m.RepoID).One(&repo); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db repo with id %d", m.RepoID)
		}
		return errors.Wrapf(err, "failed to fetch from db repo with id %d", m.RepoID)
	}

	if repo.DeletedAt != nil {
		dc.log.Warnf("Repo %d is already deleted", m.RepoID)
	}

	provider, err := dc.providerFactory.BuildForUser(gormDB, m.UserID)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	switch repo.CommitState {
	case models.RepoCommitStateDeleteInit, models.RepoCommitStateDeleteSentToQueue:
		if err := dc.deleteRepo(ctx, &repo, gormDB, provider); err != nil {
			return errors.Wrap(err, "failed to delete repo")
		}
		return nil
	default:
		return fmt.Errorf("invalid repo commit state %s for repo %#v", repo.CommitState, repo)
	}
}

func (dc DeleterConsumer) deleteRepo(ctx context.Context, repo *models.Repo,
	gormDB *gorm.DB, p provider.Provider) error {

	if err := p.DeleteRepoHook(ctx, repo.Owner(), repo.Repo(), repo.ProviderHookID); err != nil {
		if err == provider.ErrNotFound {
			dc.log.Warnf("Repo %s hook id %s was already deleted by previous run or manually by user",
				repo, repo.HookID)
		} else {
			return errors.Wrap(err, "failed to delete hook from provider")
		}
	}

	now := time.Now()
	n, err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateIn(models.RepoCommitStateDeleteInit, models.RepoCommitStateDeleteSentToQueue).
		GetUpdater().
		SetCommitState(models.RepoCommitStateDeleteDone).
		SetDeletedAt(&now).
		UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}
	if n != 1 {
		return fmt.Errorf("race condition during update repo with id %d, n=%d", repo.ID, n)
	}

	return nil
}

func (dc DeleterConsumer) consumeMessage(ctx context.Context, m *deleteMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, dc.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	return dc.run(ctx, m, gormDB)
}
