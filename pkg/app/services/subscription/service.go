package subscription

import (
	"github.com/golangci/golangci-api/pkg/app/returntypes"
	"github.com/golangci/golangci-api/pkg/endpoint/request"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"
)

type SubCreateRequest struct {
	request.OrgID

	PaymentGatewayCardToken string `json:"payment_gateway_card_token"`
	SeatsCount              int    `json:"seatsCount"`
}

func (r SubCreateRequest) FillLogContext(lctx logutil.Context) {
	r.OrgID.FillLogContext(lctx)
	if r.SeatsCount > 0 {
		lctx["seats_count"] = r.SeatsCount
	}
	// TODO(all): Decide whatever token should be logged, it's probably going to be very long string
}

type SubUpdateRequest struct {
	SubCreateRequest
	request.SubID
}

func (r SubUpdateRequest) FillLogContext(lctx logutil.Context) {
	r.SubCreateRequest.FillLogContext(lctx)
	r.SubID.FillLogContext(lctx)
}

type WrappedSubInfo struct {
	Subscription returntypes.SubInfo `json:"subscription"`
}

type Service interface {
	//url:/v1/orgs/{org_id}/subs
	List(rc *request.AuthorizedContext, reqOrg *request.OrgID) (*WrappedSubInfo, error)

	//url:/v1/orgs/{org_id}/subs/{sub_id}
	Get(rc *request.AuthorizedContext, reqSub *request.OrgSubID) (*returntypes.SubInfo, error)

	//url:/v1/orgs/{org_id}/subs method:POST
	Create(rc *request.AuthorizedContext, reqSub *SubCreateRequest) (*returntypes.SubInfo, error)

	//url:/v1/orgs/{org_id}/subs/{sub_id} method:PUT
	Update(rc *request.AuthorizedContext, reqSub *SubUpdateRequest) error

	//url:/v1/orgs/{org_id}/subs/{sub_id} method:DELETE
	Delete(rc *request.AuthorizedContext, reqSub *request.OrgSubID) error
}

func Configure() Service {
	return &basicService{}
}

type basicService struct{}

func (s *basicService) List(rc *request.AuthorizedContext, reqOrg *request.OrgID) (*WrappedSubInfo, error) {
	return nil, errors.New("not implemented")
}

func (s *basicService) Get(rc *request.AuthorizedContext, reqSub *request.OrgSubID) (*returntypes.SubInfo, error) {
	return nil, errors.New("not implemented")
}

func (s *basicService) Create(rc *request.AuthorizedContext, reqSub *SubCreateRequest) (*returntypes.SubInfo, error) {
	return nil, errors.New("not implemented")
}

func (s *basicService) Update(rc *request.AuthorizedContext, reqSub *SubUpdateRequest) error {
	return errors.New("not implemented")
}

func (s *basicService) Delete(rc *request.AuthorizedContext, reqSub *request.OrgSubID) error {
	return errors.New("not implemented")
}
