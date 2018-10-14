package session

import (
	"github.com/gorilla/sessions"
)

type RequestContext struct {
	Saver    *Saver
	Registry *sessions.Registry
}
