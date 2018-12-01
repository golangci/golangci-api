package main

import (
	"github.com/golangci/golangci-api/pkg/worker/app"
)

func main() {
	a := app.NewApp()
	a.Run()
}
