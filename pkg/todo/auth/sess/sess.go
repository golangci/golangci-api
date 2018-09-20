package sess

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	redisapi "github.com/golangci/golangci-api/pkg/db/redis"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"

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

func Get(httpReq *http.Request) (*sessions.Session, error) {
	primaryStoreOnce.Do(func() {
		primaryStore = CreateStore(90 * 24 * 3600) // 90 days
	})
	return primaryStore.Get(httpReq, sessionCookieName)
}

func GetValue(httpReq *http.Request, key string) (interface{}, error) {
	s, err := Get(httpReq)
	if err != nil {
		return nil, fmt.Errorf("can't get session: %s", err)
	}

	return s.Values[key], nil
}

func WriteOneValue(httpReq *http.Request, httpWriter http.ResponseWriter, k string, v interface{}) error {
	s, err := Get(httpReq)
	if err != nil {
		return fmt.Errorf("can't get session for request: %s", err)
	}

	s.Values[k] = v
	if err := s.Save(httpReq, httpWriter); err != nil {
		return fmt.Errorf("can't save session changes by key %q: %s", k, err)
	}

	return nil
}

func Remove(httpReq *http.Request, httpWriter http.ResponseWriter) error {
	s, err := Get(httpReq)
	if err != nil {
		return errors.Wrap(err, "can't get session for request")
	}

	s.Options.MaxAge = -1
	s.Values = make(map[interface{}]interface{})
	if err = s.Save(httpReq, httpWriter); err != nil {
		return errors.Wrap(err, "could not delete user session")
	}

	return nil
}
