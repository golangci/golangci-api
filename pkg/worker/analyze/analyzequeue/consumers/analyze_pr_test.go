package consumers

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/golangci/golangci-api/pkg/worker/test"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeRepo(t *testing.T) {
	test.MarkAsSlow(t)
	test.Init()

	prNumber := 1
	if pr := os.Getenv("PR"); pr != "" {
		var err error
		prNumber, err = strconv.Atoi(pr)
		assert.NoError(t, err)
	}
	const userID = 1

	repoOwner := "golangci"
	repoName := "golangci-worker"
	if r := os.Getenv("REPO"); r != "" {
		parts := strings.SplitN(r, "/", 2)
		repoOwner, repoName = parts[0], parts[1]
	}

	err := NewAnalyzePR().Consume(context.Background(), repoOwner, repoName,
		os.Getenv("TEST_GITHUB_TOKEN"), prNumber, "", userID, "test-guid")
	assert.NoError(t, err)
}
