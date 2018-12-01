package session

import (
	"net/http"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

type Saver struct {
	sessions []*sessions.Session
	log      logutil.Log
}

func NewSaver(log logutil.Log) *Saver {
	return &Saver{
		log: log,
	}
}

func (s *Saver) Save(sess *sessions.Session) {
	s.sessions = append(s.sessions, sess)
}

func (s Saver) FinalizeHTTP(r *http.Request, w http.ResponseWriter) error {
	for _, sess := range s.sessions {
		if err := sess.Save(r, w); err != nil {
			return errors.Wrapf(err, "can't finalize session saving for sess %#v", sess)
		}
		s.log.Infof("Session finalization: url=%s: saved session %#v", r.URL.String(), sess.Values)
	}

	return nil
}
