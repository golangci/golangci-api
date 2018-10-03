package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/golangci/golangci-api/pkg/ensuredeps"
)

func main() {
	verboseLog := flag.Bool("verbose", false, "Verbose logging")
	repoName := flag.String("repo", "", "repo name or path")
	flag.Parse()
	if *repoName == "" {
		log.Fatalf("Repo name must be set: use --repo")
	}

	r := ensuredeps.NewRunner(*verboseLog, *repoName)
	ret := r.Run()
	if err := json.NewEncoder(os.Stdout).Encode(ret); err != nil {
		log.Fatalf("Failed to JSON output result: %s", err)
	}
}
