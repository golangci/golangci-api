package request

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/jinzhu/gorm"

	"github.com/golangci/golangci-shared/pkg/logutil"
)

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
	Sess *session.Session
}
