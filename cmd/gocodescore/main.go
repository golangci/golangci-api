package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/golangci/golangci-api/internal/api/score"

	"github.com/golangci/golangci-lint/pkg/printers"
)

func main() {
	cmd := exec.Command("golangci-lint", "run", "--out-format=json", "--issues-exit-code=0")
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("Failed to run golangci-lint: %s", err)
	}

	var runRes printers.JSONResult
	if err = json.Unmarshal(out, &runRes); err != nil {
		log.Fatalf("Failed to json unmarshal golangci-lint output %s: %s", string(out), err)
	}

	calcRes := score.Calculator{}.Calc(&runRes)
	fmt.Printf("Score: %d/%d\n", calcRes.Score, calcRes.MaxScore)
	if len(calcRes.Recommendations) != 0 {
		for _, rec := range calcRes.Recommendations {
			fmt.Printf("  - get %d more score: %s\n", rec.ScoreIncrease, rec.Text)
		}
	}
}
