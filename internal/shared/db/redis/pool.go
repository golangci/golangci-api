package redis

import (
	"errors"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golangci/golangci-api/internal/shared/config"
)

func GetPool(cfg config.Config) (*redis.Pool, error) {
	redisURL, err := GetURL(cfg)
	if err != nil {
		return nil, err
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

func GetURL(cfg config.Config) (string, error) {
	if redisURL := cfg.GetString("REDIS_URL"); redisURL != "" {
		return redisURL, nil
	}

	host := cfg.GetString("REDIS_HOST")
	password := cfg.GetString("REDIS_PASSWORD")
	if host == "" || password == "" {
		return "", errors.New("no REDIS_URL or REDIS_{HOST,PASSWORD} in config")
	}

	return fmt.Sprintf("redis://h:%s@%s", password, host), nil
}
