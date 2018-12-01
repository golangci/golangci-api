package executors

import (
	"fmt"
	"os"
)

type envStore struct {
	env []string
}

func newEnvStore() *envStore {
	return &envStore{
		env: os.Environ(),
	}
}

func (e *envStore) SetEnv(k, v string) {
	e.env = append(e.env, fmt.Sprintf("%s=%s", k, v))
}
