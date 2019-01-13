package repos

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/invitations"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue/repoanalyzes"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

const createQueueID = "repos/create"

type createMessage struct {
	RepoID uint
}

func (m createMessage) LockID() string {
	return fmt.Sprintf("%s/%d", createQueueID, m.RepoID)
}

type CreatorProducer struct {
	producers.Base
}

func (cp *CreatorProducer) Register(m *producers.Multiplexer) error {
	return cp.Base.Register(m, createQueueID)
}

func (cp CreatorProducer) Put(repoID uint) error {
	return cp.Base.Put(createMessage{
		RepoID: repoID,
	})
}

type CreatorConsumer struct {
	log                      logutil.Log
	db                       *sql.DB
	cfg                      config.Config
	providerFactory          providers.Factory
	analysisLauncherQueue    *repoanalyzes.LauncherProducer
	invitationsAcceptorQueue *invitations.AcceptorProducer
}

func NewCreatorConsumer(log logutil.Log, db *sql.DB, cfg config.Config, pf providers.Factory,
	alq *repoanalyzes.LauncherProducer, invq *invitations.AcceptorProducer) *CreatorConsumer {

	return &CreatorConsumer{
		log:                      log,
		db:                       db,
		cfg:                      cfg,
		providerFactory:          pf,
		analysisLauncherQueue:    alq,
		invitationsAcceptorQueue: invq,
	}
}

func (cc CreatorConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(cc.consumeMessage, createQueueID, m, df)
}

func (cc CreatorConsumer) createHook(ctx context.Context, repo *models.Repo, p provider.Provider) error {
	// List hooks because they could be created in previous run: make run idempotent
	hooks, err := p.ListRepoHooks(ctx, repo.Owner(), repo.Repo())
	if err != nil {
		// TODO: check here and everywhere in this module for
		// unrecoverable provider errors: not found, no auth
		return errors.Wrap(err, "failed to list repo hooks")
	}

	hookCfg := provider.HookConfig{
		Name:   "web",
		Events: []string{"push", "pull_request"},
		URL: cc.cfg.GetString("GITHUB_CALLBACK_HOST") +
			fmt.Sprintf("/v1/repos/%s/hooks/%s", repo.FullName, repo.HookID),
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
			// this text goes to user when e.g. we get "repo is archived" error
			return errors.Wrapf(err, "can't save webhook to %s", p.Name())
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

	return cc.processStates(ctx, &repo, gormDB, provider)
}

//nolint:gocyclo
func (cc CreatorConsumer) processStates(ctx context.Context, repo *models.Repo, gormDB *gorm.DB, p provider.Provider) error {
	for {
		cc.log.Infof("Repo creation: handling state %s", repo.CommitState)
		switch repo.CommitState {
		case models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue:
			if err := cc.createRepo(ctx, repo, gormDB, p); err != nil {
				if !provider.IsPermanentError(err) {
					return errors.Wrap(err, "failed to create repo")
				}

				cc.log.Warnf("Failed to create repo %s: permanent provider error: %s: rollbacking it",
					repo.FullName, err)
				if uErr := cc.markRepoForRollback(repo, gormDB, err); uErr != nil {
					return errors.Wrap(uErr, "failed to mark repo for rollback")
				}

				// handle RepoCommitStateCreateRollbackInit in the next iteration
				continue
			}
			continue
		case models.RepoCommitStateCreateCreatedRepo:
			if err := cc.createRepoAnalysisStatus(ctx, repo, gormDB, p); err != nil {
				return errors.Wrap(err, "failed to create repo analysis status")
			}
			continue
		case models.RepoCommitStateCreateDone:
			return nil // terminal state
		case models.RepoCommitStateCreateRollbackInit:
			if err := cc.rollbackRepo(repo, gormDB); err != nil {
				return errors.Wrap(err, "failed to rollback repo")
			}
			continue
		case models.RepoCommitStateCreateRollbackDone:
			return nil // terminal state
		default:
			return fmt.Errorf("invalid repo commit state %s for repo %#v", repo.CommitState, repo)
		}
	}
}

func (cc CreatorConsumer) createRepoAnalysisStatusInDB(ctx context.Context, db *gorm.DB,
	p provider.Provider, r *models.Repo) (*models.RepoAnalysisStatus, error) {
	providerRepo, err := p.GetRepoByName(ctx, r.Owner(), r.Repo())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch repo %s/%s from provider",
			r.Owner(), r.Repo())
	}

	providerRepoBranch, err := p.GetBranch(ctx, r.Owner(), r.Repo(), providerRepo.DefaultBranch)
	if err != nil {
		if err == provider.ErrNotFound {
			cc.log.Warnf("Repo %s/%s has empty default branch %s",
				r.Owner(), r.Repo(), providerRepo.DefaultBranch)
		} else {
			return nil, errors.Wrapf(err, "failed to fetch repo %s/%s default branch %s from provider",
				r.Owner(), r.Repo(), providerRepo.DefaultBranch)
		}
	}

	var as models.RepoAnalysisStatus
	if providerRepoBranch != nil {
		as = models.RepoAnalysisStatus{
			DefaultBranch:     providerRepo.DefaultBranch,
			PendingCommitSHA:  providerRepoBranch.CommitSHA,
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
			IsEmpty:           true,
		}
	}
	if err = as.Create(db); err != nil {
		return nil, errors.Wrap(err, "can't create analysis status in db")
	}

	return &as, nil
}

func (cc CreatorConsumer) createRepoAnalysisStatus(ctx context.Context, r *models.Repo,
	db *gorm.DB, p provider.Provider) error {

	var as models.RepoAnalysisStatus
	fetchErr := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(r.ID).One(&as)
	if fetchErr != nil {
		if fetchErr != gorm.ErrRecordNotFound {
			return errors.Wrap(fetchErr, "failed to fetch repo analysis status from db")
		}

		ras, err := cc.createRepoAnalysisStatusInDB(ctx, db, p, r)
		if err != nil {
			return err
		}
		as = *ras
	}

	if as.HasPendingChanges { // false if empty repo
		if err := cc.analysisLauncherQueue.Put(r.ID, as.PendingCommitSHA); err != nil {
			return errors.Wrap(err, "failed to send repo to analyze queue")
		}
	}

	return cc.updateRepoCommitState(r, db, models.RepoCommitStateCreateDone)
}

func (cc CreatorConsumer) createRepo(ctx context.Context, repo *models.Repo,
	gormDB *gorm.DB, p provider.Provider) error {

	if err := cc.createHook(ctx, repo, p); err != nil {
		return err
	}

	if err := cc.addCollaborator(ctx, repo, p); err != nil {
		return err
	}

	nextState := models.RepoCommitStateCreateCreatedRepo
	err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateIn(models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue).
		GetUpdater().
		SetProviderHookID(repo.ProviderHookID).
		SetCommitState(nextState).
		UpdateRequired()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}

	repo.CommitState = nextState
	return nil
}

func (cc CreatorConsumer) updateRepoCommitState(repo *models.Repo, gormDB *gorm.DB, state models.RepoCommitState) error {
	prevState := repo.CommitState
	err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateEq(prevState).
		GetUpdater().
		SetCommitState(state).
		UpdateRequired()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}

	repo.CommitState = state
	return nil
}

func (cc CreatorConsumer) markRepoForRollback(repo *models.Repo, gormDB *gorm.DB, sourceErr error) error {
	nextState := models.RepoCommitStateCreateRollbackInit
	err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateIn(models.RepoCommitStateCreateInit, models.RepoCommitStateCreateSentToQueue).
		GetUpdater().
		SetCreateFailReason(sourceErr.Error()).
		SetCommitState(nextState).
		UpdateRequired()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}

	repo.CommitState = nextState
	return nil
}

func (cc CreatorConsumer) rollbackRepo(repo *models.Repo, gormDB *gorm.DB) error {

	nextState := models.RepoCommitStateCreateRollbackDone
	now := time.Now()
	err := models.NewRepoQuerySet(gormDB).IDEq(repo.ID).
		CommitStateEq(models.RepoCommitStateCreateRollbackInit).
		GetUpdater().
		SetCommitState(nextState).
		SetDeletedAt(&now).
		UpdateRequired()
	if err != nil {
		return errors.Wrapf(err, "failed to update repo with id %d", repo.ID)
	}

	repo.CommitState = nextState
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

func (cc CreatorConsumer) addCollaborator(ctx context.Context, repo *models.Repo, p provider.Provider) error {
	if !repo.IsPrivate {
		return nil // no need to add golangcibot as collaborator for a public repo
	}

	// p.AddCollaborator is idempotent: repetitive calls before invitation accept
	// return the same 201 response. Calls after invitation accept return 204.
	reviewerLogin := getReviewerLogin(p.Name(), cc.cfg)
	invite, err := p.AddCollaborator(ctx, repo.Owner(), repo.Repo(), reviewerLogin)
	if err != nil {
		// this text goes to user when err == ErrNeedMoreOrgSeats
		return errors.Wrapf(err, "can't add @%s as collaborator in %s",
			reviewerLogin, p.Name())
	}

	if invite.IsAlreadyCollaborator {
		cc.log.Warnf("Race condition or bug: reviewer bot is already collaborator of %s, don't send to invite queue",
			repo.FullNameWithProvider())
		return nil
	}

	cc.log.Infof("Sent invitation to reviewer bot for being collaborator for repo %s",
		repo.FullNameWithProvider())

	// p.AddCollaborator just send invitation, which our reviewer user needs to accept
	// to get access to the repo
	if err = cc.invitationsAcceptorQueue.Put(p.Name(), repo, invite.ID); err != nil {
		return errors.Wrapf(err, "failed to send task with invitation id %d for repo %s",
			invite.ID, repo.FullNameWithProvider())
	}

	return nil
}
