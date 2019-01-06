package main

import (
	app "github.com/golangci/golangci-api/pkg/api"
)

func main() {
	app := app.NewApp()
	app.RunForever()
}
