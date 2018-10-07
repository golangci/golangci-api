package endpointutil

import (
	"context"
	"time"

	"github.com/golangci/golangci-api/pkg/apierrors"
	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/request"
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/apperrors"
	"github.com/golangci/golangci-shared/pkg/logutil"
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

func makeBaseRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker, db *gorm.DB) *request.BaseContext {
	lctx := logutil.Context{}
	log = logutil.WrapLogWithContext(log, lctx)
	log = apperrors.WrapLogWithTracker(log, lctx, et)

	return &request.BaseContext{
		Ctx:       ctx,
		Log:       log,
		Lctx:      lctx,
		DB:        db,
		StartedAt: time.Now(),
	}
}

func MakeAnonymousRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker, db *gorm.DB) *request.AnonymousContext {
	return &request.AnonymousContext{
		BaseContext: *makeBaseRequestContext(ctx, log, et, db),
	}
}

func MakeAuthorizedRequestContext(ctx context.Context, log logutil.Log, et apperrors.Tracker, db *gorm.DB, sess *session.Session) (*request.AuthorizedContext, error) {
	baseCtx := makeBaseRequestContext(ctx, log, et, db)

	const userIDSessKey = "UserID"
	userIDobj := sess.GetValue(userIDSessKey)
	if userIDobj == nil {
		baseCtx.Log.Infof("No user id in session %#v", sess)
		return nil, apierrors.ErrNotAuthorized
	}

	userIDfloat := userIDobj.(float64)
	userID := uint(userIDfloat)

	var auth models.Auth
	if err := models.NewAuthQuerySet(db).UserIDEq(userID).One(&auth); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.Wrapf(err, "no user with id %d", userID)
		}

		return nil, errors.Wrapf(err, "failed to fetch user with id %d", userID)
	}

	return &request.AuthorizedContext{
		BaseContext: *baseCtx,
		Sess:        sess,
		Auth:        &auth,
	}, nil
}
