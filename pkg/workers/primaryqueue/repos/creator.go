package repos

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/golangci/golangci-api/pkg/db/gormdb"
	"github.com/golangci/golangci-api/pkg/providers/provider"
	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-api/pkg/workers/primaryqueue"

	"github.com/golangci/golangci-api/pkg/providers"

	"github.com/golangci/golangci-api/pkg/queue/producers"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"

	"gopkg.in/redsync.v1"
)

const createQueueID = "repos/create"

type createMessage struct {
	RepoID uint
}

func (m createMessage) DeduplicationID() string {
	return fmt.Sprintf("%s/%d", createQueueID, m.RepoID)
}

type CreatorProducer struct {
	q producers.Queue
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	q, err := m.NewSubqueue(createQueueID)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s subqueue", createQueueID)
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
	log             logutil.Log
	db              *sql.DB
	cfg             config.Config
	providerFactory *providers.Factory
}

func NewCreatorConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pf *providers.Factory) *CreatorConsumer {
	return &CreatorConsumer{
		log:             log,
		db:              db,
		cfg:             cfg,
		providerFactory: pf,
	}
}

func (cc CreatorConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	consumer, err := consumers.NewReflectConsumer(cc.consumeMessage, primaryqueue.ConsumerTimeout, df)
	if err != nil {
		return errors.Wrap(err, "can't make reflect consumer")
	}

	return m.RegisterConsumer(createQueueID, consumer)
}

func (cc CreatorConsumer) createHook(ctx context.Context, repo *models.Repo, p provider.Provider) error {
	// List hooks because they could be created in previous run: make run idempotent
	hooks, err := p.ListRepoHooks(ctx, repo.Owner(), repo.Repo())
	if err != nil {
		return errors.Wrap(err, "failed to list repo hooks")
	}

	hookCfg := provider.HookConfig{
		Name:   "web",
		Events: []string{"push", "pull_request"},
		URL: cc.cfg.GetString("GITHUB_CALLBACK_HOST") +
			fmt.Sprintf("/v1/repos/%s/hooks/%s", repo.Name, repo.HookID),
		ContentType: "json",
	}

	var installedHookID int
	for _, hook := range hooks {
		if hook.URL == hookCfg.URL {
			installedHookID = hook.ID
			break
		}
	}

	if installedHookID != 0 {
		repo.ProviderHookID = installedHookID
	} else {
		var hook *provider.Hook
		hook, err = p.CreateRepoHook(ctx, repo.Owner(), repo.Repo(), &hookCfg)
		if err != nil {
			return errors.Wrapf(err, "can't post hook %#v to provider", hookCfg)
		}
		repo.ProviderHookID = hook.ID
	}

	return nil
}

func (cc CreatorConsumer) run(ctx context.Context, m *createMessage, gormDB *gorm.DB) error {
	// TODO: take provider error into account and delete repo with saving error description

	var repo models.Repo
	if err := models.NewRepoQuerySet(gormDB).IDEq(m.RepoID).One(&repo); err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.Wrapf(consumers.ErrPermanent, "failed to fetch from db repo with id %d", m.RepoID)
		}
		return errors.Wrapf(err, "failed to fetch from db repo with id %d", m.RepoID)
	}

	provider, err := cc.providerFactory.BuildForUser(gormDB, repo.UserID)
	if err != nil {
		return errors.Wrap(err, "failed to build provider")
	}

	switch repo.CommitState {
	case models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue:
		if err := cc.createRepo(ctx, &repo, gormDB, provider); err != nil {
			return errors.Wrap(err, "failed to create repo")
		}
		fallthrough
	case models.RepoCommitStateCreateCreatedRepo:
		if err := cc.createRepoAnalysisStatus(ctx, &repo, gormDB, provider); err != nil {
			return errors.Wrap(err, "failed to create repo analysis status")
		}
		return nil
	case models.RepoCommitStateCreateDone:
		cc.log.Warnf("Got repo with commit state %s: %#v", repo.CommitState, repo)
		return nil
	default:
		return fmt.Errorf("invalid repo commit state %s for repo %#v", repo.CommitState, repo)
	}
}

func (cc CreatorConsumer) createRepoAnalysisStatus(ctx context.Context, r *models.Repo,
	db *gorm.DB, p provider.Provider) error {

	var as models.RepoAnalysisStatus
	fetchErr := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(r.ID).One(&as)
	if fetchErr != nil {
		if fetchErr != gorm.ErrRecordNotFound {
			return errors.Wrap(fetchErr, "failed to fetch repo analysis status from db")
		}

		providerRepo, err := p.GetRepoByName(ctx, r.Owner(), r.Repo())
		if err != nil {
			return errors.Wrapf(err, "failed to fetch repo %s/%s from provider",
				r.Owner(), r.Repo())
		}

		providerRepoBranch, err := p.GetBranch(ctx, r.Owner(), r.Repo(), providerRepo.DefaultBranch)
		if err != nil {
			if err == provider.ErrNotFound {
				cc.log.Warnf("Repo %s/%s has empty default branch %s",
					r.Owner(), r.Repo(), providerRepo.DefaultBranch)
			} else {
				return errors.Wrapf(err, "failed to fetch repo %s/%s default branch %s from provider",
					r.Owner(), r.Repo(), providerRepo.DefaultBranch)
			}
		}

		var as models.RepoAnalysisStatus
		if providerRepoBranch != nil {
			as = models.RepoAnalysisStatus{
				DefaultBranch:     providerRepo.DefaultBranch,
				PendingCommitSHA:  providerRepoBranch.HeadCommitSHA,
				HasPendingChanges: true,
				Active:            true,
				RepoID:            r.ID,
			}
		} else { // empty repo
			as = models.RepoAnalysisStatus{
				DefaultBranch:     providerRepo.DefaultBranch,
				PendingCommitSHA:  "",
				HasPendingChanges: false,
				Active:            true,
				RepoID:            r.ID,
			}
		}
		if err = as.Create(db); err != nil {
			return errors.Wrap(err, "can't create analysis status in db")
		}
	}

	// TODO: start analysis here

	n, err := models.NewRepoQuerySet(db).IDEq(r.ID).
		CommitStateEq(models.RepoCommitStateCreateCreatedRepo).
		GetUpdater().
		SetCommitState(models.RepoCommitStateCreateDone).
		UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", r.ID)
	}
	if n != 1 {
		return fmt.Errorf("race condition during update repo with id %d", r.ID)
	}

	return nil
}

func (cc CreatorConsumer) createRepo(ctx context.Context, repo *models.Repo,
	gormDB *gorm.DB, p provider.Provider) error {

	if err := cc.createHook(ctx, repo, p); err != nil {
		return err
	}

	n, err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateIn(models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue).
		GetUpdater().
		SetProviderHookID(repo.ProviderHookID).
		SetCommitState(models.RepoCommitStateCreateCreatedRepo).
		UpdateNum()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}
	if n != 1 {
		return fmt.Errorf("race condition during update repo with id %d, n=%d", repo.ID, n)
	}

	return nil
}

func (cc CreatorConsumer) consumeMessage(ctx context.Context, m *createMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, cc.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	if err = cc.run(ctx, m, gormDB); err != nil {
		return errors.Wrapf(err, "create of repo %d failed", m.RepoID)
	}

	return nil
}
