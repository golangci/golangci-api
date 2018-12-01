package main

import (
	"github.com/golangci/golangci-api/pkg/api"
)

func main() {
	app := app.NewApp()
	app.RunForever()
}
