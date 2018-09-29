package request

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/golangci/golangci-shared/pkg/logutil"
)

type Repo struct {
	Provider string `request:",url,"`
	Owner    string `request:",url,"`
	Name     string `request:",url,"`
}

func (r Repo) FullName() string {
	return strings.ToLower(fmt.Sprintf("%s/%s", r.Owner, r.Name))
}

func (r Repo) String() string {
	return fmt.Sprintf("%s/%s", r.Provider, r.FullName())
}

func (r Repo) FillLogContext(lctx logutil.Context) {
	lctx["repo"] = r.String()
}

type BodyRepo struct {
	Provider string
	Owner    string
	Name     string
}

func (r BodyRepo) FullName() string {
	return strings.ToLower(fmt.Sprintf("%s/%s", r.Owner, r.Name))
}

func (r BodyRepo) String() string {
	return fmt.Sprintf("%s/%s", r.Provider, r.FullName())
}

func (r BodyRepo) FillLogContext(lctx logutil.Context) {
	lctx["repo"] = r.String()
}

type RepoID struct {
	ID uint `request:"repoID,url,"`
}

func (r RepoID) FillLogContext(lctx logutil.Context) {
	lctx["repoID"] = strconv.Itoa(int(r.ID))
}
