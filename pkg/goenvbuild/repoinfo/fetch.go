package repoinfo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/goenvbuild/packages"
	"github.com/pkg/errors"
)

func extractAlias(prog *packages.Program) (string, error) {
	for _, p := range prog.Packages() {
		bp := p.BuildPackage()
		if bp.ImportComment == "" {
			continue
		}

		importComment := strings.TrimSuffix(bp.ImportComment, "/")
		var repoCanonicalImportPath string
		if p.Dir() == "." {
			repoCanonicalImportPath = importComment
		} else {
			if !strings.HasSuffix(importComment, p.Dir()) {
				return "", fmt.Errorf("invalid import comment %q in dir %q", importComment, p.Dir())
			}
			repoCanonicalImportPath = strings.TrimSuffix(importComment, p.Dir())
			repoCanonicalImportPath = strings.TrimSuffix(repoCanonicalImportPath, "/")
		}

		return repoCanonicalImportPath, nil
	}

	return "", nil
}

func tryExtractInfoFromGoMod() (*Info, error) {
	content, err := ioutil.ReadFile("go.mod")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open go.mod file")
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return nil, errors.New("no lines in go.mod")
	}

	firstLine := strings.TrimSpace(lines[0])
	const prefix = "module "
	if !strings.HasPrefix(firstLine, prefix) {
		return nil, fmt.Errorf("bad go.mod first line prefix: %s", firstLine)
	}

	name := strings.TrimPrefix(firstLine, prefix)
	if name == "" {
		return nil, fmt.Errorf("bad go.mod first line: empty module name: %s", firstLine)
	}

	if name[0] == '"' {
		unquotedName, err := strconv.Unquote(name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unquote module name %s", name)
		}
		name = unquotedName
	}

	return &Info{
		CanonicalImportPath:       name,
		CanonicalImportPathReason: "extracted from go.mod file",
	}, nil
}

func tryExtractInfoFromTravisYml() (*Info, error) {
	f, err := os.Open(".travis.yml")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open .travis.yml file")
	}
	defer f.Close()

	var t struct {
		GoImportPath string `yaml:"go_import_path"`
	}
	if err = yaml.NewDecoder(f).Decode(&t); err != nil {
		return nil, errors.Wrap(err, "failed to yaml decode .travis.yml")
	}

	if t.GoImportPath == "" {
		return nil, errors.New("no go_import_path directive in .travis.yml")
	}

	return &Info{
		CanonicalImportPath:       t.GoImportPath,
		CanonicalImportPathReason: "extracted from .travis.yml go_import_path directive",
	}, nil
}

func tryExtractInfoFromCircleciYml() (*Info, error) {
	path := filepath.Join(".circleci", "config.yml")
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s file", path)
	}
	defer f.Close()

	var data struct {
		Jobs struct {
			Build struct {
				WorkingDirectory string `yaml:"working_directory"`
			}
		}
	}
	if err = yaml.NewDecoder(f).Decode(&data); err != nil {
		return nil, errors.Wrapf(err, "failed to yaml decode %s", path)
	}

	yamlPath := "jobs.build.working_directory"
	wd := data.Jobs.Build.WorkingDirectory
	if wd == "" {
		return nil, fmt.Errorf("no %s directive in %s", yamlPath, path)
	}

	const prefix = "/go/src/"
	if !strings.HasPrefix(wd, prefix) {
		return nil, fmt.Errorf("bad prefix of %s: %s", yamlPath, wd)
	}

	return &Info{
		CanonicalImportPath:       strings.TrimPrefix(wd, prefix),
		CanonicalImportPathReason: fmt.Sprintf("extracted from %s %s directive", yamlPath, path),
	}, nil
}

func tryExtractInfoFromGlideYaml() (*Info, error) {
	const path = "glide.yaml"
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s file", path)
	}
	defer f.Close()

	var data struct {
		Package string
	}
	if err = yaml.NewDecoder(f).Decode(&data); err != nil {
		return nil, errors.Wrapf(err, "failed to yaml decode %s", path)
	}

	if data.Package == "" {
		return nil, fmt.Errorf("no package directive in %s", path)
	}

	return &Info{
		CanonicalImportPath:       data.Package,
		CanonicalImportPathReason: fmt.Sprintf("extracted from %s package directive", path),
	}, nil
}

//nolint:gocyclo
func Fetch(repo string, log logutil.Log) (*Info, error) {
	tryExtract := func(name string, f func() (*Info, error)) *Info {
		info, err := f()
		if err == nil {
			return info
		}

		log.Infof("Try to extract info from %s: no info: %s", name, err)
		return nil
	}

	if info := tryExtract("go.mod", tryExtractInfoFromGoMod); info != nil {
		return info, nil
	}
	if info := tryExtract("travis config", tryExtractInfoFromTravisYml); info != nil {
		return info, nil
	}
	if info := tryExtract("glide config", tryExtractInfoFromGlideYaml); info != nil {
		return info, nil
	}
	if info := tryExtract("circleci config", tryExtractInfoFromCircleciYml); info != nil {
		return info, nil
	}

	r, err := packages.NewResolver(nil, packages.StdExcludeDirRegexps, logutil.NewStderrLog("getrepoinfo"))
	if err != nil {
		return nil, errors.Wrap(err, "can't make resolver")
	}

	prog, err := r.Resolve("./...")
	if err != nil {
		return nil, errors.Wrap(err, "can't resolve")
	}

	alias, err := extractAlias(prog)
	if err != nil {
		return nil, errors.Wrap(err, "failed to extract alias")
	}
	if alias != "" && alias != repo {
		return &Info{
			CanonicalImportPath:       alias,
			CanonicalImportPathReason: "alias found in import comments",
		}, nil
	}

	if strings.ToLower(repo) != repo {
		return nil, fmt.Errorf("must set lowercased repo")
	}

	for _, p := range prog.Packages() {
		bp := p.BuildPackage()
		imports := append([]string{}, bp.Imports...)
		imports = append(imports, bp.TestImports...)
		imports = append(imports, bp.XTestImports...)

		for _, imp := range imports {
			impLower := strings.ToLower(imp)
			if imp == impLower {
				continue
			}

			if impLower == repo || strings.HasPrefix(impLower, repo+"/") {
				return &Info{
					CanonicalImportPath:       imp[:len(repo)],
					CanonicalImportPathReason: "found import of another case",
				}, nil
			}
		}
	}

	return &Info{
		CanonicalImportPath:       repo,
		CanonicalImportPathReason: "another canonical path wasn't detected",
	}, nil
}
