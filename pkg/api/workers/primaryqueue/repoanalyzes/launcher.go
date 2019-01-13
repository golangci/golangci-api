package repoanalyzes

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/providers"

	"github.com/golangci/golangci-api/pkg/worker/analyze/analyzesqueue/repoanalyzesqueue"

	"github.com/golangci/golangci-api/internal/shared/db/gormdb"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	redsync "gopkg.in/redsync.v1"
)

const launchQueueID = "repoanalyzes/launch"
const lintersVersion = "v1.10.1"

type launchMessage struct {
	RepoID    uint
	CommitSHA string
}

func (m launchMessage) LockID() string {
	return fmt.Sprintf("%s/%d", launchQueueID, m.RepoID)
}

type LauncherProducer struct {
	producers.Base
}

func (p *LauncherProducer) Register(m *producers.Multiplexer) error {
	return p.Base.Register(m, launchQueueID)
}

func (p LauncherProducer) Put(repoID uint, commitSHA string) error {
	return p.Base.Put(launchMessage{
		RepoID:    repoID,
		CommitSHA: commitSHA,
	})
}

type LauncherConsumer struct {
	log         logutil.Log
	db          *sql.DB
	runProducer *repoanalyzesqueue.Producer
	pf          providers.Factory
}

func NewLauncherConsumer(log logutil.Log, db *sql.DB, runProducer *repoanalyzesqueue.Producer, pf providers.Factory) *LauncherConsumer {
	return &LauncherConsumer{
		log:         log,
		db:          db,
		runProducer: runProducer,
		pf:          pf,
	}
}

func (c LauncherConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(c.consumeMessage, launchQueueID, m, df)
}

func (c LauncherConsumer) consumeMessage(ctx context.Context, m *launchMessage) error {
	gormDB, err := gormdb.FromSQL(ctx, c.db)
	if err != nil {
		return errors.Wrap(err, "failed to get gorm db")
	}

	return c.run(m, gormDB)
}

func (c LauncherConsumer) run(m *launchMessage, db *gorm.DB) (retErr error) {
	var as models.RepoAnalysisStatus
	if err := models.NewRepoAnalysisStatusQuerySet(db).RepoIDEq(m.RepoID).One(&as); err != nil {
		return errors.Wrapf(err, "failed to fetch repo analysis status for repo %d", m.RepoID)
	}

	tx, finishTx, err := gormdb.StartTx(db)
	if err != nil {
		return err
	}
	defer finishTx(&retErr)

	if err = c.setPendingChanges(tx, m, &as); err != nil {
		return errors.Wrap(err, "failed to set pending changes")
	}

	// TODO: check lastanalyzedat and reschedule task in queue laster

	// use Unscoped to fetch deleted repos
	var repo models.Repo
	if err = models.NewRepoQuerySet(tx.Unscoped()).IDEq(m.RepoID).One(&repo); err != nil {
		return errors.Wrapf(err, "failed to fetch repo with id %d", m.RepoID)
	}

	if err = c.launchRepoAnalysis(tx, &as, &repo); err != nil {
		return errors.Wrap(err, "failed to launch repo analysis")
	}

	return nil
}

func (c LauncherConsumer) setPendingChanges(tx *gorm.DB, m *launchMessage, as *models.RepoAnalysisStatus) error {
	n, err := models.NewRepoAnalysisStatusQuerySet(tx).
		IDEq(as.ID).
		VersionEq(as.Version).
		GetUpdater().
		SetHasPendingChanges(true).
		SetPendingCommitSHA(m.CommitSHA).
		SetVersion(as.Version + 1).
		UpdateNum()
	if err != nil {
		return errors.Wrap(err, "can't update repo analysis status after processing")
	}
	if n == 0 {
		return fmt.Errorf("got race condition updating repo analysis status on version %d->%d",
			as.Version, as.Version+1)
	}

	as.Version++
	as.PendingCommitSHA = m.CommitSHA
	as.HasPendingChanges = true
	return nil
}

func (c LauncherConsumer) launchRepoAnalysis(tx *gorm.DB, as *models.RepoAnalysisStatus, repo *models.Repo) error {
	needSendToQueue := true
	nExisting, err := models.NewRepoAnalysisQuerySet(tx).
		RepoAnalysisStatusIDEq(as.ID).CommitSHAEq(as.PendingCommitSHA).LintersVersionEq(lintersVersion).
		Count()
	if err != nil {
		return errors.Wrap(err, "can't count existing repo analyzes")
	}
	if nExisting != 0 {
		// TODO: just fix version on sending to queue
		c.log.Warnf("Can't create repo analysis because of "+
			"race condition with frequent pushes and not fixed commit in worker: %#v", *as)
		needSendToQueue = false
	}

	if needSendToQueue {
		if err = c.createNewAnalysis(tx, as, repo); err != nil {
			return errors.Wrap(err, "failed to create new analysis")
		}
	}

	n, err := models.NewRepoAnalysisStatusQuerySet(tx).
		IDEq(as.ID).
		VersionEq(as.Version).
		GetUpdater().
		SetHasPendingChanges(false).
		SetPendingCommitSHA("").
		SetVersion(as.Version + 1).
		SetLastAnalyzedAt(time.Now().UTC()).
		SetLastAnalyzedLintersVersion(lintersVersion).
		UpdateNum()
	if err != nil {
		return errors.Wrap(err, "can't update repo analysis status after processing")
	}
	if n == 0 {
		var curAS models.RepoAnalysisStatus
		if err := models.NewRepoAnalysisStatusQuerySet(tx).IDEq(as.ID).One(&curAS); err != nil {
			return errors.Wrap(err, "failed to fetch current repo analysis status")
		}
		return fmt.Errorf("got race condition updating repo analysis status on version %d->%d: %#v",
			as.Version, as.Version+1, curAS)
	}

	return nil
}

func (c LauncherConsumer) createNewAnalysis(tx *gorm.DB, as *models.RepoAnalysisStatus, repo *models.Repo) error {
	a := models.RepoAnalysis{
		RepoAnalysisStatusID: as.ID,
		AnalysisGUID:         uuid.NewV4().String(),
		Status:               "sent_to_queue",
		CommitSHA:            as.PendingCommitSHA,
		ResultJSON:           []byte("{}"),
		AttemptNumber:        1,
		LintersVersion:       lintersVersion,
	}
	if err := a.Create(tx); err != nil {
		return errors.Wrap(err, "can't create repo analysis")
	}

	pat, err := c.getPrivateAccessToken(tx, repo)
	if err != nil {
		return errors.Wrap(err, "failed to get private access token")
	}

	if err := c.runProducer.Put(repo.FullName, a.AnalysisGUID, as.DefaultBranch, pat); err != nil {
		return errors.Wrap(err, "failed to enqueue repo analysis for running")
	}

	return nil
}

func (c LauncherConsumer) getPrivateAccessToken(db *gorm.DB, repo *models.Repo) (string, error) {
	if !repo.IsPrivate {
		return "", nil
	}

	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(repo.UserID).One(&auth); err != nil {
		return "", errors.Wrapf(err, "failed to fetch auth for user id %d", repo.UserID)
	}

	return auth.PrivateAccessToken, nil
}
