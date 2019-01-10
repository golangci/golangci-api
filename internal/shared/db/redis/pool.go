package redis

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/internal/shared/config"
)

func GetPool(cfg config.Config) (*redis.Pool, error) {
	redisURL := cfg.GetString("REDIS_URL")
	if redisURL == "" {
		return nil, fmt.Errorf("config REDIS_URL isn't set")
	}

	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		TestOnBorrow: func(c redis.Conn, _ time.Time) error {
			_, pingErr := c.Do("PING")
			return pingErr
		},
		Dial: func() (redis.Conn, error) {
			return redis.DialURL(redisURL)
		},
	}, nil
}
