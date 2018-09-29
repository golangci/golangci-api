package session

import (
	"net/http"
	"strings"

	"github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/pkg/errors"

	redistore "gopkg.in/boj/redistore.v1"
)

const sessionCookieName = "s"

type Factory struct {
	store *redistore.RediStore
}

func NewFactory(redisPool *redis.Pool, cfg config.Config) (*Factory, error) {
	store, err := redistore.NewRediStoreWithPool(redisPool, []byte(cfg.GetString("SESSION_SECRET")))
	if err != nil {
		return nil, errors.Wrap(err, "can't create redis session store")
	}

	store.SetMaxAge(90 * 24 * 3600) // 90 days
	store.SetSerializer(redistore.JSONSerializer{})

	// https for dev/prod, http for testing
	d := strings.TrimPrefix(cfg.GetString("WEB_ROOT"), "https://")
	d = strings.TrimPrefix(d, "http://")
	store.Options.Domain = d
	// TODO: set httponly and secure for non-testing

	return &Factory{
		store: store,
	}, nil
}

func (f Factory) Build(httpReq *http.Request) (*Session, error) {
	gs, err := f.store.Get(httpReq, sessionCookieName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session")
	}

	return &Session{
		gs:      gs,
		httpReq: httpReq,
	}, nil
}
