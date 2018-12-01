package transportutil

import (
	"github.com/golangci/golangci-api/internal/api/session"
	"github.com/golangci/golangci-api/internal/shared/apperrors"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

type HandlerRegContext struct {
	Router          *mux.Router
	Log             logutil.Log
	ErrTracker      apperrors.Tracker
	DB              *gorm.DB
	AuthSessFactory *session.Factory
}
