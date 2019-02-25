package executors

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/pkg/errors"
)

type TempDirShell struct {
	shell
}

var _ Executor = &TempDirShell{}

func NewTempDirShell(tag string) (*TempDirShell, error) {
	tmpRoot, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return nil, errors.Wrap(err, "can't eval symlinks on /tmp")
	}

	wd, err := ioutil.TempDir(tmpRoot, fmt.Sprintf("golangci.%s", tag))
	if err != nil {
		return nil, errors.Wrap(err, "can't make temp dir")
	}

	return &TempDirShell{
		shell: *newShell(wd),
	}, nil
}

func (s TempDirShell) WorkDir() string {
	return s.wd
}

func (s *TempDirShell) SetWorkDir(wd string) {
	s.wd = wd
}

func (s TempDirShell) Clean() {
	if err := os.RemoveAll(s.wd); err != nil {
		analytics.Log(context.TODO()).Warnf("Can't remove temp dir %s: %s", s.wd, err)
	}
}

func (s TempDirShell) WithEnv(k, v string) Executor {
	eCopy := s
	eCopy.SetEnv(k, v)
	return &eCopy
}

func (s TempDirShell) WithWorkDir(wd string) Executor {
	eCopy := s
	eCopy.wd = wd
	return &eCopy
}

func (s TempDirShell) CopyFile(ctx context.Context, dst, src string) error {
	dst = filepath.Join(s.WorkDir(), dst)

	from, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("can't open %s: %s", src, err)
	}
	defer from.Close()

	to, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("can't open %s: %s", dst, err)
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return fmt.Errorf("can't copy from %s to %s: %s", src, dst, err)
	}

	return nil
}
