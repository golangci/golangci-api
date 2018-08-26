package config

import "time"

type Config interface {
	GetString(key string) string
	GetDuration(key string, def time.Duration) time.Duration
	GetInt(key string, def int) int
	GetBool(key string, def bool) bool
}
