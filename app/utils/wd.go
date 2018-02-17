package utils

import (
	"os"
	"path"
	"runtime"
)

func GetProjectRoot() string {
	if os.Getenv("GO_ENV") == "prod" {
		return "./" // we are in heroku
	}

	// we are running locally
	_, filename, _, _ := runtime.Caller(0)
	return path.Clean(path.Join(path.Dir(filename), "..", ".."))
}
