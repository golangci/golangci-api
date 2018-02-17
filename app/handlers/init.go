package handlers

import (
	"net/http"

	"github.com/golangci/golib/server/handlers/manager"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
)

func GetRoot() http.Handler {
	h := manager.GetHTTPHandler()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://golangci.com", "https://dev.golangci.com"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
	})

	n := negroni.Classic()
	n.Use(c)
	n.UseHandler(h)
	return n
}
