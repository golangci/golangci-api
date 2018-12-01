package main

import (
	"time"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/buildagent/build"
)

func main() {
	log := logutil.NewStderrLog("runner")
	log.SetLevel(logutil.LogLevelInfo)
	cfg := config.NewEnvConfig(log)

	// shutdown server after maxLifetime to prevent staling containers
	// eating all system resources
	token := cfg.GetString("TOKEN")
	r := build.NewRunner(log, token)

	maxLifetime := cfg.GetDuration("MAX_LIFETIME", 30*time.Minute)
	port := cfg.GetInt("PORT", 7000)
	if err := r.Run(port, maxLifetime); err != nil {
		log.Warnf("Runner error: %s", err)
	}
}
