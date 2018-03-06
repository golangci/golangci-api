package handlers

import (
	"net/http"
	"os"

	"github.com/golangci/golangci-api/app/internal/auth/user"
	"github.com/golangci/golib/server/context"
	"github.com/golangci/golib/server/handlers/herrors"
	"github.com/golangci/golib/server/handlers/manager"
	"github.com/rs/cors"
	"github.com/stvp/rollbar"
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

func trackError(ctx *context.C, err error) {
	fields := []*rollbar.Field{}
	u, userErr := user.GetCurrent(ctx)
	if userErr != nil {
		fields = append(fields, &rollbar.Field{
			Name: "user",
			Data: u,
		})
	}

	rollbar.RequestError("ERROR", ctx.R, err, fields...)
	ctx.L.Warnf("Tracked error to rollbar: %+v", u)
}

func Register(match string, handler manager.Handler) {
	wrappedHandler := func(ctx context.C) error {
		err := handler(ctx)
		if err != nil {
			if herr, ok := err.(herrors.HTTPError); ok && herr.Code() == http.StatusForbidden {
				// it's not an error
				return err
			}

			go trackError(&ctx, err)
		}
		return err
	}
	manager.Register(match, wrappedHandler)
}

func init() {
	rollbar.Token = os.Getenv("ROLLBAR_API_TOKEN")
	goEnv := os.Getenv("GO_ENV")
	if goEnv == "prod" {
		rollbar.Environment = "production" // defaults to "development"
	}
}
