package handlers

import (
	"net/http"

	"github.com/golangci/golangci-api/pkg/todo/errors"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
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

func Register(match string, handler manager.Handler) {
	wrappedHandler := func(ctx context.C) error {
		err := handler(ctx)
		if err != nil {
			if herr, ok := err.(herrors.HTTPError); ok && herr.Code() == http.StatusForbidden && ctx.R.URL.Path == "/v1/auth/check" {
				// it's not an error
				return err
			}

			errors.Error(&ctx, err)
		}
		return err
	}
	manager.Register(match, wrappedHandler)
}
