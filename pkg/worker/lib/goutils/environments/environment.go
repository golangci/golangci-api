package environments

type EnvSettable interface {
	SetEnv(key, value string)
}
