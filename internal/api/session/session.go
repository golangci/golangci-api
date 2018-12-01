package session

import (
	"fmt"

	"github.com/gorilla/sessions"
)

type Session struct {
	gs    *sessions.Session
	saver *Saver
}

func (s Session) GoString() string {
	return fmt.Sprintf("%#v", s.gs.Values)
}

func (s Session) GetValue(key string) interface{} {
	return s.gs.Values[key]
}

func (s *Session) Set(k string, v interface{}) {
	s.gs.Values[k] = v
	s.saver.Save(s.gs)
}

func (s *Session) Delete() {
	s.gs.Options.MaxAge = -1
	s.gs.Values = make(map[interface{}]interface{})
	s.saver.Save(s.gs)
}
