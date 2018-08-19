package repoanalyzes

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func getDurationFromEnv(key string, def time.Duration) time.Duration {
	cfgStr := os.Getenv(key)
	if cfgStr == "" {
		return def
	}

	d, err := time.ParseDuration(cfgStr)
	if err != nil {
		logrus.Errorf("Invalid %s %q: %s", key, cfgStr, err)
		return def
	}

	return d
}
