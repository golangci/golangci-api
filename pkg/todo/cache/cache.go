package cache

import (
	"os"
	"sync"
	"time"
)

type Cache interface {
	Get(key string, dest interface{}) error
	Set(key string, expireTimeout time.Duration, value interface{}) error
}

var initCacheOnce sync.Once
var cache Cache

func Get() Cache {
	initCacheOnce.Do(func() {
		cache = NewRedis(os.Getenv("REDIS_URL") + "/1")
	})
	return cache
}
