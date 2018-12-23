package sharedtest

import (
	"log"
	"net/http/httptest"
	"os"
	"path"

	"github.com/golangci/golangci-api/internal/shared/fsutil"
	app "github.com/golangci/golangci-api/pkg/api"
	"github.com/joho/godotenv"
)

type App struct {
	app              *app.App
	testserver       *httptest.Server
	fakeGithubServer *httptest.Server
}

func RunApp() *App {
	loadEnv()

	ta := App{}
	ta.initFakeGithubServer()

	deps := ta.BuildCommonDeps()

	modifiers := []app.Modifier{
		app.SetProviderFactory(deps.ProviderFactory),
	}

	ta.app = app.NewApp(modifiers...)

	ta.testserver = httptest.NewServer(ta.app.GetHTTPHandler())
	os.Setenv("GITHUB_CALLBACK_HOST", ta.testserver.URL)
	os.Setenv("WEB_ROOT", ta.testserver.URL)

	ta.app.RunEnvironment()

	return &ta
}

func loadEnv() {
	envNames := []string{".env", ".env.test"}
	for _, envName := range envNames {
		fpath := path.Join(fsutil.GetProjectRoot(), envName)
		err := godotenv.Overload(fpath)
		if err != nil {
			log.Fatalf("Can't load %s: %s", fpath, err)
		}
	}
}
