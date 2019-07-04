package consumers

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/golangci/golangci-api/internal/shared/apperrors"

	"github.com/golangci/golangci-api/internal/shared/config"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"

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

	pf := processors.NewBasicPullProcessorFactory(&processors.BasicPullConfig{})
	log := logutil.NewStderrLog("")
	cfg := config.NewEnvConfig(log)
	errTracker := apperrors.NewNopTracker()

	err := NewAnalyzePR(pf, log, errTracker, cfg).Consume(context.Background(), repoOwner, repoName,
		false, cfg.GetString("TEST_GITHUB_TOKEN"), prNumber, "", userID, "test-guid", "commit-sha")
	assert.NoError(t, err)
}
