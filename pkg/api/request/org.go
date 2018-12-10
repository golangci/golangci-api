package request

import "github.com/golangci/golangci-api/internal/shared/logutil"

type OrgID struct {
	OrgID uint `request:"org_id,urlPart,"`
}

func (o OrgID) FillLogContext(lctx logutil.Context) {
	lctx["org_id"] = o.OrgID
}

type Org struct {
	Provider string `request:",urlPart,"`
	Name     string `request:",urlPart,"`
}

func (o Org) FillLogContext(lctx logutil.Context) {
	lctx["org_provider"] = o.Provider
	lctx["org_name"] = o.Name
}

type SubID struct {
	SubID uint `request:"sub_id,urlPart,"`
}

func (s SubID) FillLogContext(lctx logutil.Context) {
	lctx["sub_id"] = s.SubID
}

type OrgSubID struct {
	OrgID
	SubID
}

func (os OrgSubID) FillLogContext(lctx logutil.Context) {
	os.OrgID.FillLogContext(lctx)
	os.SubID.FillLogContext(lctx)
}
