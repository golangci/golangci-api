package policy

import (
	"context"
	"strings"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type ActiveSubscription struct {
	log logutil.Log
	db  *gorm.DB
	cfg config.Config
}

func NewActiveSubscription(log logutil.Log, db *gorm.DB, cfg config.Config) *ActiveSubscription {
	return &ActiveSubscription{
		log: log,
		db:  db,
		cfg: cfg,
	}
}

func (s ActiveSubscription) checkExistingOrgSubscription(org *models.Org) (*models.OrgSub, error) {
	var orgSub models.OrgSub
	err := models.NewOrgSubQuerySet(s.db).OrgIDEq(org.ID).One(&orgSub)
	if err == gorm.ErrRecordNotFound {
		s.log.Infof("Active subscription for org id=%d doesn't exist", org.ID)
		return nil, ErrNoActiveSubscription
	} else if err != nil {
		return nil, errors.Wrap(err, "failed to fetch org sub from db")
	}

	if !orgSub.IsActive() {
		s.log.Warnf("Subscription id=%d is creating/deleting/updating now, can't connect the repo yet: commit state is %s",
			orgSub.ID, orgSub.CommitState)
		return nil, ErrNoActiveSubscription
	}

	if orgSub.SeatsCount == 0 {
		s.log.Warnf("Subscription id=%d has 0 seats", orgSub.ID)
		return nil, ErrNoActiveSubscription
	}

	return &orgSub, nil
}

func (s ActiveSubscription) CheckForProviderRepo(p provider.Provider, pr *provider.Repo) error {
	if !s.cfg.GetBool("NEED_CHECK_ACTIVE_SUBSCRIPTIONS", true) {
		s.log.Infof("Don't check active subscription by config")
		return nil
	}

	_, _, err := s.getActiveSub(p, pr)
	return err
}

func (s ActiveSubscription) getActiveSub(p provider.Provider, pr *provider.Repo) (*models.Org, *models.OrgSub, error) {
	var org models.Org
	qs := models.NewOrgQuerySet(s.db).ForProviderRepo(p.Name(), pr.Organization, pr.OwnerID)
	if err := qs.One(&org); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, ErrNoActiveSubscription
		}

		return nil, nil, errors.Wrapf(err, "failed to fetch org from db for repo %s", pr.FullName)
	}

	// org exists, check subscription
	sub, err := s.checkExistingOrgSubscription(&org)
	if err != nil {
		if err != ErrNoActiveSubscription {
			return nil, nil, errors.Wrapf(err, "failed to check org %s subscription", org.Name)
		}

		return nil, nil, err
	}

	return &org, sub, nil
}

func (s ActiveSubscription) doesCommitAuthorHaveActiveSub(ca *provider.CommitAuthor, paidSeats []models.OrgSeat) bool {
	if ca.Email == "" {
		s.log.Warnf("Commit %#v has no email", ca)
		return false
	}

	// emails of bots and similar service users, not real users
	serviceEmails := map[string]bool{
		"bot@renovateapp.com": true,
		"noreply@github.com":  true,
	}
	if serviceEmails[ca.Email] {
		s.log.Infof("Accepting commit from service email %s", ca.Email)
		return true
	}

	for _, paidSeat := range paidSeats {
		if strings.EqualFold(paidSeat.Email, ca.Email) {
			s.log.Infof("Matched commit %v by seat %v", ca, paidSeat)
			return true
		}
	}

	return false
}

func (s ActiveSubscription) buildPaidSeats(orgSettings *models.OrgSettings, sub *models.OrgSub) []models.OrgSeat {
	var seats = orgSettings.Seats
	if sub.SeatsCount > len(seats) {
		s.log.Warnf("Sub %d has more seats than in org settings", sub.ID)
	} else if sub.SeatsCount < len(seats) {
		truncatedSeats := seats[:sub.SeatsCount]
		s.log.Warnf("Sub %d has less seats (%d) than in org settings (%d), truncating seats to %v",
			sub.ID, sub.SeatsCount, len(seats), truncatedSeats)
		seats = truncatedSeats
	}

	return seats
}

func (s ActiveSubscription) CheckForProviderPullRequestEvent(ctx context.Context, p provider.Provider, ev *provider.PullRequestEvent) error {
	org, sub, err := s.getActiveSub(p, ev.Repo)
	if err != nil {
		return err
	}

	commits, err := p.ListPullRequestCommits(ctx, ev.Repo.Owner(), ev.Repo.Name(), ev.PullRequestNumber)
	if err != nil {
		return errors.Wrap(err, "failed to list pull request commits")
	}

	orgSettings, err := org.UnmarshalSettings()
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal org settings")
	}

	paidSeats := s.buildPaidSeats(orgSettings, sub)

	commitEmails := map[string]bool{}
	for _, commit := range commits {
		if s.doesCommitAuthorHaveActiveSub(commit.Author, paidSeats) {
			return nil
		}
		if s.doesCommitAuthorHaveActiveSub(commit.Committer, paidSeats) {
			return nil
		}

		commitEmails[commit.Author.Email] = true
		commitEmails[commit.Committer.Email] = true
	}

	s.log.Warnf("No seat for commits with emails %v in subscription %d with paid seats %#v", commitEmails, sub.ID, paidSeats)
	return ErrNoSeatInSubscription
}
