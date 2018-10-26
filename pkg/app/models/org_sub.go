package models

import (
	"fmt"

	"github.com/jinzhu/gorm"
)

type OrgSubCommitState string

const (
	OrgSubCommitStateCreateInit        OrgSubCommitState = "create/init"
	OrgSubCommitStateCreateSentToQueue OrgSubCommitState = "create/sent_to_queue"
	OrgSubCommitStateCreateCreatedRepo OrgSubCommitState = "create/created_repo"
	OrgSubCommitStateCreateDone        OrgSubCommitState = "create/done"

	OrgSubCommitStateDeleteInit        OrgSubCommitState = "delete/init"
	OrgSubCommitStateDeleteSentToQueue OrgSubCommitState = "delete/sent_to_queue"
	OrgSubCommitStateDeleteDone        OrgSubCommitState = "delete/done"
)

func (s OrgSubCommitState) IsDeleteState() bool {
	return s == OrgSubCommitStateDeleteInit || s == OrgSubCommitStateDeleteSentToQueue || s == OrgSubCommitStateDeleteDone
}

func (s OrgSubCommitState) IsCreateState() bool {
	return s == OrgSubCommitStateCreateInit || s == OrgSubCommitStateCreateSentToQueue ||
		s == OrgSubCommitStateCreateCreatedRepo || s == OrgSubCommitStateCreateDone
}

func (s OrgSubCommitState) IsDone() bool {
	return s == OrgSubCommitStateCreateDone || s == OrgSubCommitStateDeleteDone
}

//go:generate goqueryset -in org_sub.go

// gen:qs
type OrgSub struct {
	gorm.Model

	PaymentGatewayCardToken      string
	PaymentGatewayCustomerID     string
	PaymentGatewaySubscriptionID string

	BillingUserID uint
	OrgID         uint
	SeatsCount    int
	CommitState   OrgSubCommitState
}

func (s *OrgSub) GoString() string {
	return fmt.Sprintf("{OrgID: %d, ID: %d, Seats: %d, CommitState: %s}", s.OrgID, s.ID, s.SeatsCount, s.CommitState)
}

func (s OrgSub) IsDeleting() bool {
	return s.CommitState.IsDeleteState() && !s.CommitState.IsDone()
}

func (s OrgSub) IsCreating() bool {
	return s.CommitState.IsCreateState() && !s.CommitState.IsDone()
}
