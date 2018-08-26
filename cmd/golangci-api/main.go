package main

import (
	_ "github.com/golangci/golangci-api/app/handlers/auth"
	_ "github.com/golangci/golangci-api/app/handlers/events"
	_ "github.com/golangci/golangci-api/app/handlers/repos"
	"github.com/golangci/golangci-api/pkg/app"
	"github.com/golangci/golangci-api/pkg/shared"
)

func main() {
	shared.Init()
	app := app.NewApp()
	app.RunForever()
}
