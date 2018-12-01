package util

import (
	"os"
	"path/filepath"
)

func GetProjectRoot() string {
	if os.Getenv("GO_ENV") == "prod" {
		return "./" // we are in heroku
	}

	// when we run "go test" current working dir changed to /test dir
	// so we need to restore root dir
	gopath := os.Getenv("GOPATH")
	return filepath.Join(gopath, "src", "github.com", "golangci", "golangci-api")
}
