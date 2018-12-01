package main

import "github.com/golangci/golangci-api/pkg/api"

func main() {
	a := app.NewApp()
	a.RunDeadLetterConsumers()
}
