package main

import (
	"fmt"
	"net/http"

	"github.com/namsral/flag"

	"github.com/golangci/golangci-api/app/handlers"
	_ "github.com/golangci/golangci-api/app/handlers/auth"
	_ "github.com/golangci/golangci-api/app/handlers/events"
	_ "github.com/golangci/golangci-api/app/handlers/repos"
	"github.com/golangci/golangci-api/app/internal/shared"

	log "github.com/sirupsen/logrus"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 3000, "listen port")
	flag.Parse()

	shared.Init()

	listenHTTP(port)
}

func listenHTTP(port int) {
	h := handlers.GetRoot()

	log.Infof("Listening HTTP on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), h))
}
