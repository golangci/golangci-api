package main

import (
	"github.com/golangci/golangci-api/pkg/goenv"
	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/golangci/golangci-shared/pkg/logutil"
)

func main() {
	log := logutil.NewStderrLog("config")
	cfg := config.NewEnvConfig(log)
	p := goenv.NewPreparer(cfg)
	p.RunAndPrint()
}
