package config

type Service struct {
	ProjectPath string `mapstructure:"project-path"`

	GolangciLintVersion string `mapstructure:"golangci-lint-version"`
	Prepare             []string
}

type FullConfig struct {
	Service Service
}
