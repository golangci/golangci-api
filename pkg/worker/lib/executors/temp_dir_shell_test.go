package executors

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTempDirShellWithEnv(t *testing.T) {
	ts, err := NewTempDirShell(t.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, ts.wd)
	assert.Equal(t, os.Environ(), ts.env)

	defer ts.Clean()

	tse := ts.WithEnv("k", "v").(*TempDirShell)
	assert.NotEmpty(t, ts.wd)
	assert.Equal(t, ts.wd, tse.wd) // check was saved

	assert.Equal(t, os.Environ(), ts.env) // check didn't change
	assert.Equal(t, append(os.Environ(), "k=v"), tse.env)
}

func exists(t *testing.T, path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}

	if os.IsNotExist(err) {
		return false
	}

	assert.NoError(t, err)
	return true
}

func TestTempDirShellClean(t *testing.T) {
	ts, err := NewTempDirShell(t.Name())
	assert.NoError(t, err)

	assert.True(t, exists(t, ts.WorkDir()))
	ts.Clean()
	assert.False(t, exists(t, ts.WorkDir()))
}
