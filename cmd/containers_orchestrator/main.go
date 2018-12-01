package main

import (
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/buildagent/containers"
)

func main() {
	log := logutil.NewStderrLog("orchestrator")
	log.SetLevel(logutil.LogLevelInfo)
	cfg := config.NewEnvConfig(log)

	// shutdown server after maxLifetime to prevent staling containers
	// eating all system resources
	token := cfg.GetString("TOKEN")
	r := containers.NewOrchestrator(log, token)

	port := cfg.GetInt("PORT", 8001)
	if err := r.Run(port); err != nil {
		log.Warnf("Orchestrator running error: %s", err)
	}
}
