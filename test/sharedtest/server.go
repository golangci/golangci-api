package sharedtest

import (
	"log"
	"net/http/httptest"
	"path"
	"sync"

	appPkg "github.com/golangci/golangci-api/pkg/app"
	"github.com/golangci/golangci-api/pkg/app/shared"
	"github.com/golangci/golangci-api/pkg/app/utils"
	"github.com/joho/godotenv"
)

var server *httptest.Server
var serverOnce sync.Once

var envLoadOnce sync.Once
var app *appPkg.App

func GetApp() *appPkg.App {
	return app
}

func initServer() {
	serverOnce.Do(func() {
		app = appPkg.NewApp()
		app.RegisterHandlers()
		app.RunMigrations()
		app.RunConsumers()
		server = httptest.NewServer(appPkg.GetRoot())
	})
}

func initEnv() {
	envLoadOnce.Do(func() {
		envNames := []string{".env", ".env.test"}
		for _, envName := range envNames {
			fpath := path.Join(utils.GetProjectRoot(), envName)
			err := godotenv.Overload(fpath)
			if err != nil {
				log.Fatalf("Can't load %s: %s", fpath, err)
			}
		}

		shared.Init()
	})
}
