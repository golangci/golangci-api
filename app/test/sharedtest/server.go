package sharedtest

import (
	"log"
	"net/http/httptest"
	"path"
	"sync"

	"github.com/golangci/golangci-api/app/handlers"
	"github.com/golangci/golangci-api/app/internal/shared"
	"github.com/golangci/golangci-api/app/utils"
	"github.com/joho/godotenv"
)

var server *httptest.Server
var serverOnce sync.Once

var envLoadOnce sync.Once

func initServer() {
	serverOnce.Do(func() {
		server = httptest.NewServer(handlers.GetRoot())
	})
}

func initEnv() {
	envLoadOnce.Do(func() {
		envNames := []string{".env", ".env.test"}
		for _, envName := range envNames {
			fpath := path.Join(utils.GetProjectRoot(), envName)
			err := godotenv.Overload(fpath)
			if err != nil {
				log.Fatalf("Can't load %s: %s", envName, err)
			}
		}

		shared.Init()
	})
}
