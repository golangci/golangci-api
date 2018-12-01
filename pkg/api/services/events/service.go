package events

import (
	sharedevents "github.com/golangci/golangci-api/internal/api/events"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/request"
)

type Request struct {
	Name    string
	Payload map[string]interface{}
}

func (r Request) FillLogContext(lctx logutil.Context) {
	lctx["event_name"] = r.Name
	for k, v := range r.Payload {
		lctx[k] = v
	}
}

type Service interface {
	//url:/v1/events/analytics method:POST
	TrackEvent(rc *request.AuthorizedContext, req *Request) error
}

type BasicService struct{}

func (s BasicService) TrackEvent(rc *request.AuthorizedContext, req *Request) error {
	sharedevents.NewAuthenticatedTracker(int(rc.Auth.UserID)).Track(rc.Ctx, req.Name, req.Payload)
	return nil
}
