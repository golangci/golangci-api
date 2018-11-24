package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/golangci/golangci-api/pkg/goenv/repoinfo"
	"github.com/pkg/errors"
)

func main() {
	repo := flag.String("repo", "", "repo path")
	flag.Parse()
	if *repo == "" {
		log.Fatal("Set --repo flag")
	}

	if err := printRepoInfo(*repo); err != nil {
		log.Fatal(err)
	}
}

func printRepoInfo(repo string) error {
	var ret interface{}
	info, err := repoinfo.Fetch(repo)
	if err != nil {
		ret = struct {
			Error string
		}{
			Error: err.Error(),
		}
	} else {
		ret = info
	}

	if err = json.NewEncoder(os.Stdout).Encode(ret); err != nil {
		return errors.Wrap(err, "can't json marshal")
	}

	return nil
}
