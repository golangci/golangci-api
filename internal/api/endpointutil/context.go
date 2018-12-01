package endpointutil

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/auth"
	"github.com/golangci/golangci-api/pkg/api/models"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type contextKey string

const (
	contextKeyRequestContext contextKey = "endpoint/requestContext"
	contextKeyError          contextKey = "endpoint/error"
)

func RequestContext(ctx context.Context) request.Context {
	rc := ctx.Value(contextKeyRequestContext)
	if rc == nil {
		return nil
	}
	return rc.(request.Context)
}

func StoreRequestContext(ctx context.Context, rc request.Context) context.Context {
	return context.WithValue(ctx, contextKeyRequestContext, rc)
}

func StoreError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, contextKeyError, err)
}

func Error(ctx context.Context) error {
	v := ctx.Value(contextKeyError)
	if v == nil {
		return nil
	}

	return v.(error)
}

func makeBaseRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker,
	db *gorm.DB, sctx *session.RequestContext) *request.BaseContext {

	lctx := logutil.Context{}
	log = logutil.WrapLogWithContext(log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, et)

	return &request.BaseContext{
		Ctx:       ctx,
		Log:       log,
		Lctx:      lctx,
		DB:        db,
		StartedAt: time.Now(),
		SessCtx:   sctx,
	}
}

func MakeAnonymousRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker,
	db *gorm.DB, sctx *session.RequestContext) *request.AnonymousContext {

	return &request.AnonymousContext{
		BaseContext: *makeBaseRequestContext(ctx, log, et, db, sctx),
	}
}

func MakeAuthorizedRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker,
	db *gorm.DB, sf *session.Factory, sctx *session.RequestContext) (*request.AuthorizedContext, error) {

	authSess, err := sf.Build(sctx, auth.SessType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build auth sess")
	}

	authModel, err := auth.Get(authSess, sctx, db)
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := models.NewUserQuerySet(db).IDEq(authModel.UserID).One(&user); err != nil {
		return nil, errors.Wrapf(err, "failed to fetch user %d from db", authModel.UserID)
	}

	baseCtx := makeBaseRequestContext(ctx, log, et, db, sctx)
	baseCtx.Lctx["user_id"] = authModel.UserID
	baseCtx.Lctx["email"] = user.Email
	baseCtx.Lctx["provider_login"] = authModel.Login

	return &request.AuthorizedContext{
		BaseContext: *baseCtx,
		Auth:        authModel,
		User:        &user,
		AuthSess:    authSess,
	}, nil
}
