package transportutil

import (
	"github.com/golangci/golangci-api/pkg/session"
	"github.com/golangci/golangci-shared/pkg/apperrors"
	"github.com/golangci/golangci-shared/pkg/logutil"
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
