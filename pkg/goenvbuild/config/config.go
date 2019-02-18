package config

type Service struct {
	ProjectPath string `mapstructure:"project-path"`

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
