package request

import (
	"context"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-shared/pkg/logutil"
)

type Headers map[string]string

func (h Headers) Get(k string) string {
	return h[strings.ToLower(k)]
}

type Context interface {
	RequestStartedAt() time.Time
	Logger() logutil.Log
}

type BaseContext struct {
	Ctx  context.Context
	Log  logutil.Log
	Lctx logutil.Context
	DB   *gorm.DB

	StartedAt time.Time
	Headers   Headers
}

func (ctx BaseContext) RequestStartedAt() time.Time {
	return ctx.StartedAt
}

func (ctx BaseContext) Logger() logutil.Log {
	return ctx.Log
}

type AnonymousContext struct {
	BaseContext
}

type AuthorizedContext struct {
	BaseContext

	Auth *models.Auth
	User *models.User
	Sess *session.Session
}
