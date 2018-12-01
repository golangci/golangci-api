package request

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/jinzhu/gorm"
)

type Context interface {
	RequestStartedAt() time.Time
	Logger() logutil.Log
	SessContext() *session.RequestContext
}

type BaseContext struct {
	Ctx  context.Context
	Log  logutil.Log
	Lctx logutil.Context
	DB   *gorm.DB

	StartedAt time.Time

	SessCtx *session.RequestContext
}

func (ctx BaseContext) RequestStartedAt() time.Time {
	return ctx.StartedAt
}

func (ctx BaseContext) Logger() logutil.Log {
	return ctx.Log
}

func (ctx BaseContext) SessContext() *session.RequestContext {
	return ctx.SessCtx
}

type AnonymousContext struct {
	BaseContext
}

type AuthorizedContext struct {
	BaseContext

	Auth     *models.Auth
	User     *models.User
	AuthSess *session.Session
}

func (ac AuthorizedContext) ToAnonumousContext() *AnonymousContext {
	return &AnonymousContext{
		BaseContext: ac.BaseContext,
	}
}
