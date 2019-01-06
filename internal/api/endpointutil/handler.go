package endpointutil

import (
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/api/auth"
	"github.com/jinzhu/gorm"
)

type HandlerRegContext struct {
	Authorizer *auth.Authorizer
	Log        logutil.Log
	ErrTracker apperrors.Tracker
	Cfg        config.Config
	DB         *gorm.DB
}
