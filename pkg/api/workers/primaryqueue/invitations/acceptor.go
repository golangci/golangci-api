package invitations

import (
	"context"
	"fmt"

	"github.com/golangci/golangci-api/internal/shared/providers/provider"

	"github.com/golangci/golangci-api/pkg/api/models"

	"github.com/golangci/golangci-api/internal/shared/providers"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/queue/consumers"
	"github.com/golangci/golangci-api/internal/shared/queue/producers"
	"github.com/golangci/golangci-api/pkg/api/workers/primaryqueue"
	"github.com/pkg/errors"
	redsync "gopkg.in/redsync.v1"
)

const acceptQueueID = "repos/collaboration/invite/accept"

type acceptMessage struct {
	Owner, Repo  string
	Provider     string
	InvitationID int
}

func (m acceptMessage) LockID() string {
	return fmt.Sprintf("%s/%s/%s/%s%d", acceptQueueID, m.Provider,
		m.Owner, m.Repo, m.InvitationID)
}

type AcceptorProducer struct {
	producers.Base
}

func (p *AcceptorProducer) Register(m *producers.Multiplexer) error {
	return p.Base.Register(m, acceptQueueID)
}

func (p AcceptorProducer) Put(provider string, repo models.UniversalRepo, invitationID int) error {
	return p.Base.Put(acceptMessage{
		Owner:        repo.Owner(),
		Repo:         repo.Repo(),
		Provider:     provider,
		InvitationID: invitationID,
	})
}

type AcceptorConsumer struct {
	log logutil.Log
	cfg config.Config
	pf  providers.Factory
}

func NewAcceptorConsumer(log logutil.Log, cfg config.Config, pf providers.Factory) *AcceptorConsumer {
	return &AcceptorConsumer{
		log: log,
		cfg: cfg,
		pf:  pf,
	}
}

func (c AcceptorConsumer) Register(m *consumers.Multiplexer, df *redsync.Redsync) error {
	return primaryqueue.RegisterConsumer(c.consumeMessage, acceptQueueID, m, df)
}

func (c AcceptorConsumer) consumeMessage(ctx context.Context, m *acceptMessage) error {
	if err := c.run(ctx, m); err != nil {
		return errors.Wrapf(err, "accept of invitation %d for %s/%s/%s failed",
			m.InvitationID, m.Provider, m.Owner, m.Repo)
	}

	return nil
}

func (c AcceptorConsumer) run(ctx context.Context, m *acceptMessage) (retErr error) {
	// TODO: in case of error save error reason to db and show it on a report page

	reviewerToken := c.cfg.GetString("GITHUB_REVIEWER_ACCESS_TOKEN") // TODO: support all providers
	if reviewerToken == "" {
		return errors.New("GITHUB_REVIEWER_ACCESS_TOKEN wasn't set")
	}

	p, err := c.pf.BuildForToken(m.Provider, reviewerToken)
	if err != nil {
		return errors.Wrap(err, "failed to build VCS provider")
	}

	if err = p.AcceptRepoInvitation(ctx, m.InvitationID); err != nil {
		if errors.Cause(err) != provider.ErrNotFound {
			return errors.Wrap(err, "failed to accept invitation")
		}
		c.log.Warnf("Invitation %d for repo %s/%s/%s was already accepted", m.InvitationID, m.Provider, m.Owner, m.Repo)
	}

	// do extra check just for bug-catching
	if _, err = p.GetRepoByName(ctx, m.Owner, m.Repo); err != nil {
		return errors.Wrapf(err, "failed to check repo %s/%s access", m.Owner, m.Repo)
	}

	c.log.Infof("Successfully accepted and checked invitation %d for repo %s/%s/%s",
		m.InvitationID, m.Provider, m.Owner, m.Repo)
	return nil
}
