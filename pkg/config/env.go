package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/logutil"
)

type EnvConfig struct {
	log logutil.Log
}

func NewEnvConfig(log logutil.Log) *EnvConfig {
	return &EnvConfig{
		log: log,
	}
}

func (c EnvConfig) GetString(key string) string {
	return c.getValue(key)
}

func (c EnvConfig) getValue(key string) string {
	return os.Getenv(strings.ToUpper(key))
}

func (c EnvConfig) GetDuration(key string, def time.Duration) time.Duration {
	cfgStr := c.getValue(key)
	if cfgStr == "" {
		return def
	}

	d, err := time.ParseDuration(cfgStr)
	if err != nil {
		c.log.Warnf("Config: invalid %s %q: %s", key, cfgStr, err)
		return def
	}

	return d

}

func (c EnvConfig) GetInt(key string, def int) int {
	cfgStr := c.getValue(key)
	if cfgStr == "" {
		return def
	}

	v, err := strconv.Atoi(cfgStr)
	if err != nil {
		c.log.Warnf("Config: invalid %s %q: %s", key, cfgStr, err)
		return def
	}

	return v
}

func (c EnvConfig) GetBool(key string, def bool) bool {
	cfgStr := c.getValue(key)
	if cfgStr == "" {
		return def
	}

	if cfgStr == "1" || cfgStr == "true" {
		return true
	}

	if cfgStr == "0" || cfgStr == "false" {
		return false
	}

	c.log.Warnf("Config: invalid %s %q", key, cfgStr)
	return def
}
