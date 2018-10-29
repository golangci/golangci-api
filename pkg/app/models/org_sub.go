package models

import (
	"fmt"

	"github.com/jinzhu/gorm"
)

type OrgSubCommitState string

const (
	OrgSubCommitStateCreateInit        OrgSubCommitState = "create/init"
	OrgSubCommitStateCreateSentToQueue OrgSubCommitState = "create/sent_to_queue"
	OrgSubCommitStateCreateCreatedSub  OrgSubCommitState = "create/created_sub"
	OrgSubCommitStateCreateDone        OrgSubCommitState = "create/done"

	OrgSubCommitStateUpdateInit        OrgSubCommitState = "update/init"
	OrgSubCommitStateUpdateSentToQueue OrgSubCommitState = "update/sent_to_queue"
	OrgSubCommitStateUpdateDone        OrgSubCommitState = "update/done"

	OrgSubCommitStateDeleteInit        OrgSubCommitState = "delete/init"
	OrgSubCommitStateDeleteSentToQueue OrgSubCommitState = "delete/sent_to_queue"
	OrgSubCommitStateDeleteDone        OrgSubCommitState = "delete/done"
)

func (s OrgSubCommitState) IsDeleteState() bool {
	return s == OrgSubCommitStateDeleteInit || s == OrgSubCommitStateDeleteSentToQueue || s == OrgSubCommitStateDeleteDone
}

func (s OrgSubCommitState) IsCreateState() bool {
	return s == OrgSubCommitStateCreateInit || s == OrgSubCommitStateCreateSentToQueue ||
		s == OrgSubCommitStateCreateCreatedSub || s == OrgSubCommitStateCreateDone
}

func (s OrgSubCommitState) IsUpdateState() bool {
	return s == OrgSubCommitStateUpdateInit || s == OrgSubCommitStateUpdateSentToQueue || s == OrgSubCommitStateUpdateDone
}

func (s OrgSubCommitState) IsDone() bool {
	return s == OrgSubCommitStateCreateDone || s == OrgSubCommitStateDeleteDone || s == OrgSubCommitStateUpdateDone
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

func (s OrgSub) IsUpdating() bool {
	return s.CommitState.IsUpdateState() && !s.CommitState.IsDone()
}

func (s OrgSub) IsActive() bool {
	return s.CommitState.IsDone() && (s.CommitState.IsCreateState() || s.CommitState.IsUpdateState())
}
