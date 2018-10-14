package ensuredeps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

type Message struct {
	Kind string
	Text string
}

type Result struct {
	Success  bool
	Warnings []Message `json:",omitempty"`

	UsedTool       string
	UsedToolReason string
}

type tool struct {
	name    string
	syncCmd []string
}

var defaultTool = tool{
	name:    "go get",
	syncCmd: []string{"go", "get", "-t", "./..."},
}

func (t tool) sync(ctx context.Context) error {
	return exec.CommandContext(ctx, t.syncCmd[0], t.syncCmd[1:]...).Run()
}

type Runner struct {
	res           *Result
	depTool       *tool
	depToolReason string

	repoName   string
	verboseLog bool
}

func NewRunner(verboseLog bool, repoName string) *Runner {
	return &Runner{
		res:        &Result{},
		verboseLog: verboseLog,
		repoName:   repoName,
	}
}

func (r *Runner) warn(kind, text string) {
	r.res.Warnings = append(r.res.Warnings, Message{
		Kind: kind,
		Text: text,
	})
	if r.verboseLog {
		logrus.Warnf("%s: %s", kind, text)
	}
}

func (r *Runner) infof(format string, args ...interface{}) {
	if r.verboseLog {
		logrus.Infof(format, args...)
	}
}

func (r Runner) Run() *Result {
	hasGoFiles, err := r.hasDirGoFilesRecursively(".")
	if err != nil {
		r.warn("check repo", err.Error())
	} else if !hasGoFiles {
		r.warn("check repo", "no Go files")
		return r.res
	}

	r.infof("current dir has go files")

	ctx := context.Background()
	hasFilledVendor, err := r.hasFilledVendorDir()
	if err != nil {
		r.warn("check vendor dir", err.Error())
	}
	r.infof("has filled vendor dir: %t", hasFilledVendor)

	r.res.Success = true
	if !hasFilledVendor || r.checkDeps(ctx) != nil {
		if err = r.syncDeps(ctx); err != nil {
			r.warn("sync deps", err.Error())
			r.res.Success = false
		}
		r.res.UsedTool = r.depTool.name
		r.res.UsedToolReason = r.depToolReason
	} else {
		r.res.UsedTool = "no tool"
		r.res.UsedToolReason = "vendor dir exists"
	}

	return r.res
}

func (r *Runner) syncDeps(ctx context.Context) error {
	r.infof("syncing deps...")
	detectedTool, reason, err := r.detectTool()
	if err != nil {
		r.warn("detect tool", err.Error())
		detectedTool = &defaultTool
		reason = "internal failure to use other tools"
	}
	r.depTool = detectedTool
	r.depToolReason = reason
	r.infof("detected tool is %s (%s)", detectedTool.name, reason)

	if err = detectedTool.sync(ctx); err != nil {
		if detectedTool == &defaultTool {
			return errors.Wrapf(err, "'%s' failed", detectedTool.name) // nowhere to fallback from default tool
		}
		r.warn(detectedTool.name, err.Error())

		if err = defaultTool.sync(ctx); err != nil {
			return errors.Wrapf(err, "fallback to '%s' failed", defaultTool.name)
		}

		r.depTool = &defaultTool
		r.depToolReason = fmt.Sprintf("fallback from '%s' to '%s'", detectedTool.name, defaultTool.name)
	}
	r.infof("synced deps")

	depsCheckErr := r.checkDeps(ctx)
	if depsCheckErr != nil {
		if r.depTool == &defaultTool {
			return errors.Wrap(depsCheckErr, "deps check failed")
		}

		if err = defaultTool.sync(ctx); err != nil {
			return errors.Wrapf(err, "fallback to '%s' failed", defaultTool.name)
		}

		r.depTool = &defaultTool
		r.depToolReason = fmt.Sprintf("fallback from '%s' to '%s' after deps check", detectedTool.name, defaultTool.name)

		if err = r.checkDeps(ctx); err != nil {
			return errors.Wrap(err, "check deps")
		}

		r.warn("check deps", depsCheckErr.Error())
		return nil
	}

	return nil
}

func (r Runner) checkDeps(ctx context.Context) error {
	r.infof("checking deps...")
	out, _ := exec.CommandContext(ctx, "golangci-lint", "run", "--no-config", "--disable-all", "-E", "typecheck").CombinedOutput()
	outStr := string(out)
	lines := strings.Split(outStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.ToLower(line))
		if strings.Contains(line, "could not import") && !strings.Contains(line, r.repoName) {
			r.infof("deps checking: bad: %s", line)
			return errors.New(line)
		}
	}

	r.infof("deps checking ok")
	return nil
}

func (r Runner) hasFilledVendorDir() (bool, error) {
	const vendorDir = "vendor"
	_, err := os.Stat(vendorDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "failed to check 'vendor' dir")
	}

	// vendor dir exists, check does it have Go files
	hasGoFiles, err := r.hasDirGoFilesRecursively(vendorDir)
	if err != nil {
		return false, errors.Wrap(err, "failed to check does 'vendor' dir have Go files")
	}

	return hasGoFiles, nil
}

func (r Runner) hasDirGoFilesRecursively(dir string) (bool, error) {
	hasGoFiles := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if hasGoFiles {
				return filepath.SkipDir // exit early
			}
			return nil
		}

		if strings.HasSuffix(info.Name(), ".go") {
			hasGoFiles = true
		}
		return nil
	})
	if err != nil {
		return false, errors.Wrapf(err, "dir %s walking failed", dir)
	}

	return hasGoFiles, nil
}

func (r Runner) detectTool() (*tool, string, error) {
	configs := []struct {
		filePath string
		tool     tool
	}{
		{
			filePath: "Gopkg.toml",
			tool: tool{
				name:    "dep",
				syncCmd: []string{"dep", "ensure"},
			},
		},
		{
			filePath: "glide.yaml",
			tool: tool{
				name:    "glide",
				syncCmd: []string{"glide", "install"},
			},
		},
		{
			filePath: "vendor/vendor.json",
			tool: tool{
				name:    "govendor",
				syncCmd: []string{"govendor", "sync"},
			},
		},
		{
			filePath: "Godeps/Godeps.json",
			tool: tool{
				name:    "godep",
				syncCmd: []string{"godep", "restore"},
			},
		},
	}
	for _, cfg := range configs {
		_, err := os.Stat(cfg.filePath)
		if os.IsNotExist(err) {
			continue
		}

		if err != nil {
			return nil, "", errors.Wrapf(err, "failed to check path %s", cfg.filePath)
		}

		return &cfg.tool, fmt.Sprintf("%s was found", cfg.filePath), nil
	}

	return &defaultTool, "", nil
}
