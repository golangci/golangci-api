package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
)

const keyPrefix = "cache/"

type Redis struct {
	pool *redis.Pool
}

func NewRedis(redisURL string) *Redis {
	return &Redis{
		pool: &redis.Pool{
			MaxIdle:     10,
			IdleTimeout: 240 * time.Second,
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				_, pingErr := c.Do("PING")
				return pingErr
			},
			Dial: func() (redis.Conn, error) {
				return redis.DialURL(redisURL)
			},
		},
	}
}

func (r Redis) Get(key string, dest interface{}) error {
	key = keyPrefix + key

	conn := r.pool.Get()
	defer conn.Close()

	var data []byte
	data, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return nil // Cache miss
		}
		return fmt.Errorf("error getting key %s: %v", key, err)
	}

	if err = json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("can't unmarshal json from redis: %s", err)
	}

	return nil
}

func (r Redis) Set(key string, expireTimeout time.Duration, value interface{}) error {
	key = keyPrefix + key

	valueBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("can't json marshal value: %s", err)
	}

	conn := r.pool.Get()
	defer conn.Close()

	_, err = conn.Do("SETEX", key, int(expireTimeout/time.Second), valueBytes)
	if err != nil {
		v := string(valueBytes)
		if len(v) > 15 {
			v = v[0:12] + "..."
		}
		return fmt.Errorf("error setting key %s to %s: %v", key, v, err)
	}

	return nil
}
