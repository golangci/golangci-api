package main

import "github.com/golangci/golangci-api/pkg/app"

func main() {
	a := app.NewApp()
	a.RunDeadLetterConsumers()
}
