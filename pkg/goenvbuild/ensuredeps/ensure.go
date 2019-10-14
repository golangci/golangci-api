package ensuredeps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenvbuild/command"

	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/pkg/errors"
)

type Message struct {
	Kind string
	Text string
}

type Result struct {
	Success bool

	UsedTool       string
	UsedToolReason string
}

type tool struct {
	name    string
	syncCmd []string
	syncEnv []string
}

var defaultTool = tool{
	name:    "go get",
	syncCmd: []string{"go", "get", "-d", "-t", "./..."},
	syncEnv: []string{"GO111MODULE=off"}, // checksum troubles
}

func (t tool) sync(ctx context.Context, r *command.StreamingRunner) error {
	for _, env := range t.syncEnv {
		r = r.WithEnvPair(env)
	}

	out, err := r.Run(ctx, t.syncCmd[0], t.syncCmd[1:]...)
	if err != nil {
		return errors.Wrapf(err, "command failed: %s", out)
	}

	return nil
}

type Runner struct {
	res           *Result
	depTool       *tool
	depToolReason string

	log logutil.Log
	cr  *command.StreamingRunner
}

func NewRunner(log logutil.Log, cr *command.StreamingRunner) *Runner {
	return &Runner{
		res: &Result{},
		log: log,
		cr:  cr,
	}
}

func (r Runner) Run(ctx context.Context, repoName string) *Result {
	hasGoFiles, err := r.hasDirGoFilesRecursively(".")
	if err != nil {
		r.log.Warnf("Failed to check does repo has Go files: %s", err)
	} else if !hasGoFiles {
		r.log.Warnf("Repo doesn't have Go files")
		return r.res
	}

	r.log.Infof("Current dir has go files")

	hasFilledVendor, err := r.hasFilledVendorDir()
	if err != nil {
		r.log.Warnf("Failed to check vendor dir: %s", err)
	}
	if hasFilledVendor {
		r.log.Infof("Found filled vendor dir")
	} else {
		r.log.Infof("Filled vendor dir wasn't found")
	}

	r.res.Success = true
	if !hasFilledVendor || r.checkDeps(ctx, repoName) != nil {
		if err = r.syncDeps(ctx, repoName); err != nil {
			r.log.Warnf("Failed to sync deps")
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

func (r *Runner) syncDeps(ctx context.Context, repoName string) error {
	r.log.Infof("Finding dependency management tool...")
	detectedTool, reason, err := r.detectTool()
	if err != nil {
		r.log.Warnf("Failed to detect tool: %s", err)
		detectedTool = &defaultTool
		reason = "internal failure to use other tools"
	}
	r.depTool = detectedTool
	r.depToolReason = reason

	r.log.Infof("Detected tool is %s (%s)", detectedTool.name, reason)
	r.log.Infof("Syncing deps...")

	if err = detectedTool.sync(ctx, r.cr); err != nil {
		if detectedTool == &defaultTool {
			return errors.Wrapf(err, "'%s' failed", detectedTool.name) // nowhere to fallback from default tool
		}
		r.log.Warnf("%s failed: %s", detectedTool.name, err)

		if err = defaultTool.sync(ctx, r.cr); err != nil {
			r.log.Infof("Fallback to the defaul tool %s failed: %s", defaultTool.name, err)
			return errors.Wrapf(err, "fallback to '%s' failed", defaultTool.name)
		}

		r.depTool = &defaultTool
		r.depToolReason = fmt.Sprintf("fallback from '%s' to '%s'", detectedTool.name, defaultTool.name)
	}
	r.log.Infof("Synced deps")

	depsCheckErr := r.checkDeps(ctx, repoName)
	if depsCheckErr != nil {
		if r.depTool == &defaultTool {
			return errors.Wrap(depsCheckErr, "deps check failed")
		}

		if err = defaultTool.sync(ctx, r.cr); err != nil {
			return errors.Wrapf(err, "fallback to '%s' failed", defaultTool.name)
		}

		r.depTool = &defaultTool
		r.depToolReason = fmt.Sprintf("fallback from '%s' to '%s' after deps check", detectedTool.name, defaultTool.name)

		if err = r.checkDeps(ctx, repoName); err != nil {
			return errors.Wrap(err, "check deps")
		}

		r.log.Warnf("Failed to check deps: %s", depsCheckErr)
		return nil
	}

	return nil
}

func (r Runner) checkDeps(ctx context.Context, repoName string) error {
	r.log.Infof("Checking deps...")
	out, _ := r.cr.Run(ctx, "golangci-lint", "run", "--no-config", "--disable-all", "-E", "typecheck", "--timeout=5m")
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.ToLower(line))
		if (strings.Contains(line, "could not import") && !strings.Contains(line, repoName)) ||
			strings.Contains(line, "cannot find package") {

			r.log.Infof("Deps checking: bad: %s", line)
			return errors.New(line)
		}
	}

	r.log.Infof("Deps checking ok")
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
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
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
			filePath: "go.mod",
			tool: tool{
				name:    "go mod",
				syncCmd: []string{"go", "mod", "vendor"},
				syncEnv: []string{"GO111MODULE=on"},
			},
		},
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
