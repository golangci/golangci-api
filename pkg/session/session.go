package session

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

type httpWriteCallback func(httpWriter http.ResponseWriter) error

type Session struct {
	gs        *sessions.Session
	httpReq   *http.Request
	callbacks []httpWriteCallback
}

func (s Session) GoString() string {
	return fmt.Sprintf("%#v", s.gs.Values)
}

func (s Session) GetValue(key string) interface{} {
	return s.gs.Values[key]
}

func (s Session) Set(k string, v interface{}) {
	s.gs.Values[k] = v
	s.callbacks = append(s.callbacks, func(httpWriter http.ResponseWriter) error {
		if err := s.gs.Save(s.httpReq, httpWriter); err != nil {
			return errors.Wrapf(err, "can't save session changes by key %q", k)
		}
		return nil
	})
}

func (s Session) Delete() {
	s.gs.Options.MaxAge = -1
	s.gs.Values = make(map[interface{}]interface{})
	s.callbacks = append(s.callbacks, func(httpWriter http.ResponseWriter) error {
		if err := s.gs.Save(s.httpReq, httpWriter); err != nil {
			return errors.Wrap(err, "could not delete user session")
		}
		return nil
	})
}

func (s Session) RunCallbacks(httpWriter http.ResponseWriter) error {
	for _, cb := range s.callbacks {
		if err := cb(httpWriter); err != nil {
			return err
		}
	}

	return nil
}
