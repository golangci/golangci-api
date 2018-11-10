package main

import (
	"log"

	"github.com/golangci/golangci-api/pkg/app"
)

func main() {
	a := app.NewApp()
	a.InitQueue()
	if err := a.RecoverAnalyzes(); err != nil {
		log.Fatalf("Failed to recover analyzes: %s", err)
	}
}
