package main

import (
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild"
)

func main() {
	log := logutil.NewStderrLog("config")
	cfg := config.NewEnvConfig(log)
	p := goenv.NewPreparer(cfg)
	p.RunAndPrint()
}
