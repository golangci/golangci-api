package main

import (
	"github.com/golangci/golangci-api/pkg/app"
)

func main() {
	app := app.NewApp()
	app.RunForever()
}
