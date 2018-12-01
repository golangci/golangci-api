package executors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golangci/golangci-api/pkg/worker/test"
	"github.com/stretchr/testify/assert"
)

func testNewRemoteShell() *RemoteShell {
	return NewRemoteShell(
		os.Getenv("REMOTE_SHELL_USER"),
		os.Getenv("REMOTE_SHELL_HOST"),
		os.Getenv("REMOTE_SHELL_KEY_FILE_PATH"),
	)
}

func TestRemoteShellEnv(t *testing.T) {
	t.SkipNow()

	test.Init()
	s := testNewRemoteShell().WithEnv("TEST_KEY", "TEST_VALUE")

	out, err := s.Run(context.Background(), "printenv", "TEST_KEY")
	assert.NoError(t, err)
	assert.Equal(t, "TEST_VALUE", strings.TrimSpace(out))
}

func TestRemoteShellClean(t *testing.T) {
	t.SkipNow()

	test.Init()
	s := testNewRemoteShell()
	assert.NoError(t, s.SetupTempWorkDir(context.Background()))

	s.Clean() // must remove temp work dir

	_, err := s.Run(context.Background(), "test", "!", "-e", s.tempWorkDir) // returns 0 only if dir doesn't exist
	assert.NoError(t, err)
}

func TestRemoteShellInWorkDir(t *testing.T) {
	t.SkipNow()

	test.Init()
	s := testNewRemoteShell()
	assert.NoError(t, s.SetupTempWorkDir(context.Background()))
	defer s.Clean()

	testSubdir := "testdir"
	_, err := s.Run(context.Background(), "mkdir", filepath.Join(s.WorkDir(), testSubdir))
	assert.NoError(t, err)

	wd := filepath.Join(s.tempWorkDir, testSubdir)
	out, err := s.WithWorkDir(wd).Run(context.Background(), "pwd")
	assert.NoError(t, err)
	assert.Equal(t, wd, strings.TrimSpace(out))
}
