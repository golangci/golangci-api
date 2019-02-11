package goenv

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golangci/golangci-api/pkg/goenvbuild/command"

	"github.com/golangci/golangci-api/pkg/goenvbuild/ensuredeps"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	goenvconfig "github.com/golangci/golangci-api/pkg/goenvbuild/config"
	"github.com/golangci/golangci-api/pkg/goenvbuild/logger"
	"github.com/golangci/golangci-api/pkg/goenvbuild/repoinfo"
	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var availableGolangciLintVersions = map[int]map[int][]int{
	1: {
		10: {-1, 1, 2},
		11: {-1, 1, 2, 3},
		12: {-1, 1, 2, 3, 4, 5},
		13: {-1, 1, 2},
		14: {0},
	},
}

const defaultGolangciLintVersion = "1.14.x"

type Preparer struct {
	cfg config.Config
}

func NewPreparer(cfg config.Config) *Preparer {
	return &Preparer{cfg: cfg}
}

func runStepGroup(resLog *result.Log, name string, f func(sg *result.StepGroup, logger logutil.Log) error) error {
	sg := resLog.AddStepGroup(name)
	defer sg.Finish()

	logger := logger.NewStepGroupLogger(sg)

	startedAt := time.Now()
	err := f(sg, logger)
	sg.Duration = time.Since(startedAt)

	if err != nil {
		lastStep := sg.Steps[len(sg.Steps)-1]
		lastStep.AddError(err.Error())
		return errors.Wrapf(err, "%s failed", name)
	}

	return nil
}

func (p Preparer) RunAndPrint() {
	needStreamToOutput := !p.cfg.GetBool("FORMAT_JSON", false)
	res := p.run(needStreamToOutput)
	res.Finish()

	if !needStreamToOutput {
		if err := json.NewEncoder(os.Stdout).Encode(res); err != nil {
			log.Fatalf("Failed to json encode result: %s", err)
		}
		return
	}
}

//nolint:gocyclo
func (p Preparer) run(needStreamToOutput bool) *result.Result {
	ctx := context.Background()
	timeout := p.cfg.GetDuration("TIMEOUT", time.Minute*3)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var logger *log.Logger
	if needStreamToOutput {
		logger = log.New(os.Stdout, "", 0)
	}
	res := &result.Result{
		Log: result.NewLog(logger),
	}

	saveErr := func(err error) *result.Result {
		res.Error = err.Error()
		return res
	}

	// load config
	err := runStepGroup(res.Log, "load config", func(sg *result.StepGroup, log logutil.Log) error {
		cfg, err := p.tryLoadConfig(sg, log)
		if err != nil {
			return err
		}
		res.ServiceConfig = *cfg
		return nil
	})
	if err != nil {
		return saveErr(err)
	}

	// find the project path
	projectPath := strings.ToLower(p.cfg.GetString("REPO"))
	err = runStepGroup(res.Log, "find the project path", func(sg *result.StepGroup, log logutil.Log) error {
		if projectPath == "" {
			sg.AddStep("check the default project path")
			const errText = "must set REPO environment variable to project path, e.g. REPO=github.com/project/repo"
			return errors.New(errText)
		}

		if !strings.HasPrefix(projectPath, "github.com/") {
			sg.AddStep("check the default project path")
			const errText = "currently we work only with github.com repos: " +
				"for repo 'github.com/project/repo' (even if it's imported as 'mycorp.com/project') " +
				"need to set REPO=github.com/project/repo"
			return errors.New(errText)
		}

		projectPath = p.findProjectPath(sg, log, projectPath, res.ServiceConfig.ProjectPath)
		return nil
	})
	if err != nil {
		return saveErr(err)
	}

	runner := command.NewStreamingRunner(res.Log)

	// setup work dir
	err = runStepGroup(res.Log, "setup work dir", func(sg *result.StepGroup, log logutil.Log) error {
		sg.AddStep("build GOPATH")
		log.Infof("Use GOPATH=%s", p.gopath())
		res.Environment = map[string]string{
			"GOPATH": p.gopath(),
		}

		runner = runner.WithEnv("GOPATH", p.gopath())

		newWorkDir, wdErr := p.setupWorkDir(ctx, sg, log, projectPath, runner)
		if wdErr != nil {
			return wdErr
		}

		res.WorkDir = newWorkDir
		runner = runner.WithWD(newWorkDir)
		return nil
	})
	if err != nil {
		return saveErr(err)
	}

	// setup git
	privateAccessToken := p.cfg.GetString("PRIVATE_ACCESS_TOKEN")
	if privateAccessToken != "" {
		err = runStepGroup(res.Log, "setup git for private dependencies", func(sg *result.StepGroup, log logutil.Log) error {
			return p.setupGit(ctx, sg, runner, privateAccessToken)
		})
		if err != nil {
			return saveErr(err)
		}
	}

	// print environment
	err = runStepGroup(res.Log, "print environment", func(sg *result.StepGroup, log logutil.Log) error {
		return p.printEnvironment(ctx, sg, runner)
	})
	if err != nil {
		return saveErr(err)
	}

	// setup golangci-lint - do it after preparation to disallow overwriting golangci-lint version by user-defined commands
	err = runStepGroup(res.Log, "setup golangci-lint", func(sg *result.StepGroup, log logutil.Log) error {
		version, setupErr := p.setupGolangciLint(ctx, sg, log, &res.ServiceConfig, runner)
		if setupErr != nil {
			return setupErr
		}

		res.GolangciLintVersion = version
		return nil
	})
	if err != nil {
		return saveErr(err)
	}

	// prepare repo
	err = runStepGroup(res.Log, "prepare repo", func(sg *result.StepGroup, log logutil.Log) error {
		return p.runPreparation(ctx, sg, log, &res.ServiceConfig, projectPath, runner)
	})
	if err != nil {
		return saveErr(err)
	}

	if !p.cfg.GetBool("RUN", false) { // the option RUN is enabled only for manual testing
		return res
	}

	// run golangci-lint
	err = runStepGroup(res.Log, "run golangci-lint", func(sg *result.StepGroup, log logutil.Log) error {
		r := runner.WithEnv("GOLANGCI_COM_RUN", "1")
		return p.runGolangciLint(ctx, sg, r)
	})
	if err != nil {
		return saveErr(err)
	}

	return res
}

type version struct {
	major int
	minor int
	patch *int // nil - any, -1 - major.minor, 0/1/2/... - major.minor.patch
}

func (v version) String() string {
	s := fmt.Sprintf("%d.%d", v.major, v.minor)
	if v.patch == nil {
		return s + ".x"
	}
	if *v.patch == -1 {
		return s
	}

	return fmt.Sprintf("%s.%d", s, *v.patch)
}

func (v version) StringWithV() string {
	return "v" + v.String()
}

func parseVersion(v string) (*version, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return nil, fmt.Errorf("bad count of dots in version %q", v)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse version's major part %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse version's minor elem %q", parts[1])
	}

	var patch *int
	if len(parts) == 3 {
		if parts[2] != "x" {
			patch = new(int)
			*patch, err = strconv.Atoi(parts[2])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse version's patch elem %q", parts[2])
			}
		}
	} else {
		patch = new(int)
		*patch = -1
	}

	return &version{
		major: major,
		minor: minor,
		patch: patch,
	}, nil
}

func (p Preparer) runGolangciLint(ctx context.Context, sg *result.StepGroup, runner *command.StreamingRunner) error {
	cmd := "golangci-lint"
	args := []string{"run", "-v", "--deadline=5m"}
	sg.AddStepCmd(cmd, args...)
	_, err := runner.Run(ctx, cmd, args...)
	return err
}

func (p Preparer) printEnvironment(ctx context.Context, sg *result.StepGroup, runner *command.StreamingRunner) error {
	goCmd := "go"

	versionArgs := []string{"version"}
	sg.AddStepCmd(goCmd, versionArgs...)
	_, err := runner.Run(ctx, goCmd, versionArgs...)
	if err != nil {
		return err
	}

	envArgs := []string{"env"}
	sg.AddStepCmd(goCmd, envArgs...)
	_, err = runner.Run(ctx, goCmd, envArgs...)
	if err != nil {
		return err
	}

	return nil
}

func (p Preparer) setupGolangciLint(ctx context.Context, sg *result.StepGroup, log logutil.Log,
	cfg *goenvconfig.Service, r *command.StreamingRunner) (string, error) {

	parsedVersion, err := p.parseGolangciLintVersion(sg, log, cfg)
	if err != nil {
		return "", err
	}

	neededVersion, err := p.findGolangciLintVersion(parsedVersion, sg)
	if err != nil {
		return "", err
	}
	log.Infof("Using golangci-lint v%s", neededVersion.String())

	if err = p.installGolangciLint(ctx, neededVersion, sg, log, r); err != nil {
		return "", err
	}

	return neededVersion.String(), nil
}

func (p Preparer) findInstalledGolangciLintVersion(ctx context.Context, sg *result.StepGroup, r *command.StreamingRunner) (string, error) {
	sg.AddStep("finding installed version: golangci-lint --version")
	out, err := r.Run(ctx, "golangci-lint", "--version")
	if err != nil {
		return "", err
	}

	// output example:
	// golangci-lint has version 1.12.2 built from 898ae4d on 2018-11-11T06:43:11Z
	var version string
	_, err = fmt.Sscanf(out, "golangci-lint has version %s built", &version)
	if err != nil {
		return "", err
	}

	return version, nil
}

func (p Preparer) installGolangciLint(ctx context.Context, v *version, sg *result.StepGroup,
	log logutil.Log, r *command.StreamingRunner) error {
	installedVersion, err := p.findInstalledGolangciLintVersion(ctx, sg, r)
	if err != nil {
		log.Warnf("Failed to find installed golangci-lint version, downloading needed version: %s", err)
	}

	if installedVersion == v.String() {
		sg.AddStep("installing golangci-lint " + v.StringWithV())
		log.Infof("golangci-lint of needed version is installed by default, no need to download")
		return nil
	}

	// TODO: should we use script from master?
	// TODO: install by wget-ing release archive
	const shellCmdFmt = "curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | " +
		"sh -s -- -b $GOPATH/bin %s"
	shellCmd := fmt.Sprintf(shellCmdFmt, v.StringWithV())
	sg.AddStep(shellCmd)

	_, err = r.Run(ctx, "sh", "-c", shellCmd)
	if err != nil {
		return err
	}

	return nil
}

func (p Preparer) findGolangciLintVersion(parsedVersion *version, sg *result.StepGroup) (*version, error) {
	sg.AddStep("find golangci-lint version")

	minorVersions := availableGolangciLintVersions[parsedVersion.major]
	if minorVersions == nil {
		return nil, fmt.Errorf("no available major version %d", parsedVersion.major)
	}

	patchVersions := minorVersions[parsedVersion.minor]
	if patchVersions == nil {
		return nil, fmt.Errorf("no available minor version %d", parsedVersion.minor)
	}

	ret := *parsedVersion
	if parsedVersion.patch == nil { // major.minor.x, use the latest patch version
		ret.patch = new(int)
		*ret.patch = patchVersions[len(patchVersions)-1]
		return &ret, nil
	}

	for _, v := range patchVersions {
		if v == *parsedVersion.patch {
			return &ret, nil
		}
	}

	return nil, fmt.Errorf("no available patch version %d", *parsedVersion.patch)
}

func (p Preparer) parseGolangciLintVersion(sg *result.StepGroup, log logutil.Log, cfg *goenvconfig.Service) (*version, error) {
	sg.AddStep("parse golangci-lint version from config")

	if cfg.GolangciLintVersion != "" {
		v, err := parseVersion(cfg.GolangciLintVersion)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse version %q", cfg.GolangciLintVersion)
		}
		log.Infof("Parsed version %q from config", cfg.GolangciLintVersion)
		return v, nil
	}

	v, err := parseVersion(defaultGolangciLintVersion)
	if err != nil {
		panic(err)
	}

	log.Infof("No golangci-lint version in config, use default: %q", defaultGolangciLintVersion)
	return v, nil
}

func (p Preparer) setupGit(ctx context.Context, sg *result.StepGroup,
	runner *command.StreamingRunner, privateAccessToken string) error {

	cmd := "git"
	overridePattern := fmt.Sprintf(`url.https://%s@github.com/.insteadOf`, privateAccessToken)
	cmdArgs := []string{"config", "--global", overridePattern, "https://github.com/"}
	sg.AddStepCmd(cmd, cmdArgs...)

	_, err := runner.Run(ctx, cmd, cmdArgs...)
	if err != nil {
		return errors.Wrap(err, "failed to setup git overrides")
	}

	return nil
}

func (p Preparer) runPreparation(ctx context.Context, sg *result.StepGroup, log logutil.Log,
	cfg *goenvconfig.Service, projectPath string, runner *command.StreamingRunner) error {

	if len(cfg.Prepare) == 0 {
		return p.runDefaultPreparation(ctx, sg, log, projectPath, runner)
	}

	return p.runUserDefinedPreparation(ctx, sg, cfg.Prepare, runner)
}

func (p Preparer) runDefaultPreparation(ctx context.Context, sg *result.StepGroup, log logutil.Log, projectPath string, r *command.StreamingRunner) error {
	sg.AddStep("fetch dependencies")
	runner := ensuredeps.NewRunner(log, r)
	res := runner.Run(ctx, projectPath)

	if res.Success {
		reason := res.UsedToolReason
		if reason != "" {
			reason = " (" + reason + ")"
		}
		log.Infof("Successfully fetched dependecies by [%s]%s", res.UsedTool, reason)
		return nil
	}

	log.Warnf("Failed to fetch dependecies")
	return nil
}

func (p Preparer) runUserDefinedPreparation(ctx context.Context, sg *result.StepGroup,
	steps []string, r *command.StreamingRunner) error {
	for _, step := range steps {
		sg.AddStep(step)
		if _, err := r.Run(ctx, "bash", "-c", step); err != nil {
			return errors.Wrapf(err, "failed to run command %q", step)
		}
	}

	return nil
}

func (p Preparer) gopath() string {
	gopath := p.cfg.GetString("GOPATH")
	if gopath != "" {
		return gopath
	}

	return "/go"
}

func (p Preparer) setupWorkDir(ctx context.Context, sg *result.StepGroup, log logutil.Log, projectAddr string, r *command.StreamingRunner) (string, error) {
	projectDir := filepath.Join(p.gopath(), "src", projectAddr)

	sg.AddStepCmd("mkdir", "-p", projectDir)
	if err := os.MkdirAll(projectDir, os.ModePerm); err != nil {
		return "", err
	}

	if p.cfg.GetBool("DEBUG", false) {
		sg.AddStepCmd("rm", "-r", projectDir)
		if err := os.RemoveAll(projectDir); err != nil {
			log.Warnf("Failed to remove %s: %s", projectDir, err)
		}
	}

	copyDest := projectDir + string(filepath.Separator)
	sg.AddStepCmd("cp", "-R", ".", copyDest)
	if _, err := r.Run(ctx, "cp", "-R", ".", copyDest); err != nil {
		return "", err
	}

	sg.AddStepCmd("cd", projectDir)
	if err := os.Chdir(projectDir); err != nil {
		return "", err
	}

	return projectDir, nil
}

func (p Preparer) findProjectPath(sg *result.StepGroup, log logutil.Log, defaultProjectPath, configProjectPath string) string {
	if configProjectPath != "" {
		sg.AddStep("take the project path from config")
		log.Infof("Use the project path from config: %q", configProjectPath)
		return configProjectPath
	}

	sg.AddStep("detect the project path in code")

	info, err := repoinfo.Fetch(defaultProjectPath, log)
	if err != nil {
		log.Warnf("Failed to find the project path: %s", err)
		log.Infof("Use the default project path %q", defaultProjectPath)
		return defaultProjectPath
	}

	if info.CanonicalImportPath != defaultProjectPath {
		log.Infof("Found project path is %q, reason: %s", info.CanonicalImportPath, info.CanonicalImportPathReason)
		return info.CanonicalImportPath
	}

	log.Infof("Project path is the default one %q, reason: %s", info.CanonicalImportPath, info.CanonicalImportPathReason)
	return defaultProjectPath
}

func (p Preparer) tryLoadConfig(sg *result.StepGroup, log logutil.Log) (*goenvconfig.Service, error) {
	sg.AddStep("search config file")

	viper.SetConfigName(".golangci")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Infof("No config file was found")
			return &goenvconfig.Service{}, nil
		}

		return nil, errors.Wrap(err, "can't read viper config")
	}
	log.Infof("Found config file %q", viper.ConfigFileUsed())

	sg.AddStep("parse config")
	var cfg goenvconfig.FullConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config by viper")
	}

	sg.AddStep("validate config")
	if err := p.validateConfig(&cfg.Service); err != nil {
		return nil, err
	}

	return &cfg.Service, nil
}

func (p Preparer) validateConfig(cfg *goenvconfig.Service) error {
	for _, cmd := range cfg.Prepare {
		if strings.TrimSpace(cmd) == "" {
			return fmt.Errorf("prepare command %q is empty", cmd)
		}
	}

	return nil
}
