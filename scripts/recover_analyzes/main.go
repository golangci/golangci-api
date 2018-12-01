package main

import (
	"log"

	"github.com/golangci/golangci-api/pkg/api"
)

func main() {
	a := app.NewApp()
	if err := a.RecoverAnalyzes(); err != nil {
		log.Fatalf("Failed to recover analyzes: %s", err)
	}
}
