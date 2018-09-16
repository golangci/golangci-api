package sess

import (
	"fmt"
	"os"
	"strings"
	"sync"

	redisapi "github.com/golangci/golangci-api/pkg/db/redis"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"

	"github.com/golangci/golib/server/context"
	redistore "gopkg.in/boj/redistore.v1"

	"github.com/gorilla/sessions"
)

var primaryStore *redistore.RediStore
var primaryStoreOnce sync.Once

const sessionCookieName = "s"

func CreateStore(maxAge int) *redistore.RediStore {
	log := logutil.NewStderrLog("create sess store")
	pool, err := redisapi.GetPool(config.NewEnvConfig(log))
	if err != nil {
		log.Fatalf("Can't get redis pool: %s", err)
	}

	store, err := redistore.NewRediStoreWithPool(pool, []byte(os.Getenv("SESSION_SECRET")))
	if err != nil {
		log.Fatalf("Can't create redis session store: %s", err)
	}

	log.Infof("Successfully created redis session store with maxAge %d", maxAge)

	store.SetMaxAge(maxAge)
	store.SetSerializer(redistore.JSONSerializer{})

	// https for dev/prod, http for testing
	d0 := strings.TrimPrefix(os.Getenv("WEB_ROOT"), "https://")
	d := strings.TrimPrefix(d0, "http://")
	store.Options.Domain = d
	// TODO: set httponly and secure for non-testing

	return store
}

func Get(ctx *context.C) (*sessions.Session, error) {
	primaryStoreOnce.Do(func() {
		primaryStore = CreateStore(90 * 24 * 3600) // 90 days
	})
	return primaryStore.Get(ctx.R, sessionCookieName)
}

func GetValue(ctx *context.C, key string) (interface{}, error) {
	s, err := Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get session: %s", err)
	}

	return s.Values[key], nil
}

func WriteOneValue(ctx *context.C, k string, v interface{}) error {
	s, err := Get(ctx)
	if err != nil {
		return fmt.Errorf("can't get session for request: %s", err)
	}

	s.Values[k] = v
	if err := s.Save(ctx.R, ctx.W); err != nil {
		return fmt.Errorf("can't save session changes by key %q: %s", k, err)
	}

	return nil
}

func Remove(ctx *context.C) error {
	s, err := Get(ctx)
	if err != nil {
		return fmt.Errorf("can't get session for request: %s", err)
	}

	s.Options.MaxAge = -1
	s.Values = make(map[interface{}]interface{})
	if err = s.Save(ctx.R, ctx.W); err != nil {
		return fmt.Errorf("could not delete user session: %s", err)
	}

	return nil
}
