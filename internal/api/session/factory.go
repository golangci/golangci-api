package session

import (
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/pkg/errors"
	redistore "gopkg.in/boj/redistore.v1"
)

type Factory struct {
	store *redistore.RediStore
	cfg   config.Config
}

func NewFactory(redisPool *redis.Pool, cfg config.Config, maxAge time.Duration) (*Factory, error) {
	sessSecret := cfg.GetString("SESSION_SECRET")
	if sessSecret == "" {
		return nil, errors.New("SESSION_SECRET isn't set")
	}

	store, err := redistore.NewRediStoreWithPool(redisPool, []byte(sessSecret))
	if err != nil {
		return nil, errors.Wrap(err, "can't create redis session store")
	}

	store.SetMaxAge(int(maxAge / time.Second))
	store.SetSerializer(redistore.JSONSerializer{})

	f := Factory{
		store: store,
		cfg:   cfg,
	}
	f.updateOptions()

	return &f, nil
}

func (f *Factory) updateOptions() {
	f.store.Options.Domain = f.cfg.GetString("COOKIE_DOMAIN")
	// TODO: set httponly and secure for non-testing
}

func (f *Factory) Build(ctx *RequestContext, sessType string) (*Session, error) {
	f.updateOptions() // cfg could have changed

	gs, err := ctx.Registry.Get(f.store, sessType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session")
	}

	return &Session{
		gs:    gs,
		saver: ctx.Saver,
	}, nil
}
