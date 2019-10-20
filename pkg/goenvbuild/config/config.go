package config

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type Service struct {
	ProjectPath   string   `mapstructure:"project-path"`
	AnalyzedPaths []string `mapstructure:"analyzed-paths"`

	GolangciLintVersion string `mapstructure:"golangci-lint-version"`
	Prepare             []string
	SuggestedChanges    SuggestedChangesConfig `mapstructure:"suggested-changes"`
}

type SuggestedChangesConfig struct {
	Disabled bool
}

type FullConfig struct {
	Service Service
}

func (cfg *Service) validateAnalyzedPaths() error {
	for _, path := range cfg.AnalyzedPaths {
		if strings.HasPrefix(path, "/") {
			return fmt.Errorf("path %q is invalid: only relative paths are allowed", path)
		}

		path = strings.TrimSuffix(path, "/...")
		if strings.Contains(path, "..") {
			return fmt.Errorf("path %q is invalid: analyzing of parent dirs (..) isn't allowed", path)
		}
	}

	return nil
}

func (cfg *Service) GetValidatedAnalyzedPaths() ([]string, error) {
	defaultPaths := []string{"./..."}
	if cfg == nil {
		return defaultPaths, nil
	}

	if err := cfg.validateAnalyzedPaths(); err != nil {
		return nil, errors.Wrap(err, "failed to validate service config")
	}

	if len(cfg.AnalyzedPaths) != 0 {
		return cfg.AnalyzedPaths, nil
	}

	return defaultPaths, nil
}
