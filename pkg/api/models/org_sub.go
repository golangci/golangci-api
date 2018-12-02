package models

import (
	"fmt"

	"github.com/golangci/golangci-api/internal/api/apierrors"

	"github.com/jinzhu/gorm"
)

type OrgSubCommitState string

const (
	OrgSubCommitStateCreateInit        OrgSubCommitState = "create/init"
	OrgSubCommitStateCreateSentToQueue OrgSubCommitState = "create/sent_to_queue"
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
	return s == OrgSubCommitStateCreateInit || s == OrgSubCommitStateCreateSentToQueue || s == OrgSubCommitStateCreateDone
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

	PaymentGatewayName           string
	PaymentGatewayCardToken      string
	PaymentGatewayCustomerID     string
	PaymentGatewaySubscriptionID string

	BillingUserID uint
	OrgID         uint
	SeatsCount    int
	PricePerSeat  string
	CommitState   OrgSubCommitState

	IdempotencyKey string
	Version        int
	CancelURL      string
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

func (u OrgSubUpdater) UpdateRequired() error {
	n, err := u.UpdateNum()
	if err != nil {
		return err
	}

	if n == 0 {
		return apierrors.NewRaceConditionError("data was changed in parallel request")
	}

	return nil
}
