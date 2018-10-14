package main

import (
	"github.com/golangci/golangci-api/pkg/app"
	"github.com/golangci/golangci-api/pkg/app/shared"
)

func main() {
	shared.Init()
	app := app.NewApp()
	app.RunForever()
}
