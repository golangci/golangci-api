package processors

import (
	"errors"
	"os"
	"strings"

	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

var (
	errNothingToAnalyze = errors.New("nothing to analyze")
)

type IgnoredError struct {
	Status        github.Status
	StatusDesc    string
	IsRecoverable bool
}

func (e IgnoredError) Error() string {
	return e.StatusDesc
}

func escapeErrorText(text string, secrets map[string]string) string {
	ret := text
	for secret, replacement := range secrets {
		ret = strings.Replace(ret, secret, replacement, -1)
	}

	return ret
}

func buildSecrets() map[string]string {
	const hidden = "{hidden}"
	ret := map[string]string{}

	for _, kv := range os.Environ() {
		parts := strings.Split(kv, "=")
		if len(parts) != 2 {
			continue
		}

		v := parts[1]
		if len(v) >= 6 {
			ret[v] = hidden
		}
	}

	return ret
}
