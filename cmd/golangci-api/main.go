package main

import (
	"flag"
	"fmt"
	"os"

	app "github.com/golangci/golangci-api/pkg/api"
)

var (
	// Populated during a build
	version = ""
	commit  = ""
	date    = ""
)

func main() {
	printVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *printVersion {
		fmt.Printf("golangci-api has version %s built from %s on %s\n", version, commit, date)
		os.Exit(0)
	}

	app := app.NewApp()
	app.RunForever()
}
