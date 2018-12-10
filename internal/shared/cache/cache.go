package cache

import (
	"time"
)

type Cache interface {
	Get(key string, dest interface{}) error
	Set(key string, expireTimeout time.Duration, value interface{}) error
}
