package processors

import (
	"os"
	"strings"

	"github.com/golangci/golangci-api/pkg/goenvbuild/result"
)

func escapeText(text string, secretableCtx secretable) string {
	secrets := buildSecrets(secretableCtx.secrets()...)

	ret := text
	for secret, replacement := range secrets {
		ret = strings.Replace(ret, secret, replacement, -1)
	}

	return ret
}

type secretable interface {
	secrets() []string
}

//nolint:gocyclo
func buildSecrets(vars ...string) map[string]string {
	const minSecretValueLen = 6

	const hidden = "{hidden}"
	ret := map[string]string{}
	for _, v := range vars {
		if len(v) >= minSecretValueLen {
			ret[v] = hidden
		}
	}

	for _, kv := range os.Environ() {
		parts := strings.Split(kv, "=")
		if len(parts) != 2 {
			continue
		}

		k := parts[0]
		if k == "APP_NAME" ||
			k == "ADMIN_GITHUB_LOGIN" ||
			k == "GITHUB_REVIEWER_LOGIN" ||
			k == "WEB_ROOT" ||
			k == "GOROOT" ||
			k == "GOPATH" {
			// not secret
			continue
		}

		v := parts[1]
		if len(v) >= minSecretValueLen {
			ret[v] = hidden
		}
	}

	return ret
}

func escapeBuildLog(buildLog *result.Log, s secretable) {
	for _, group := range buildLog.Groups {
		group.Name = escapeText(group.Name, s)
		for _, step := range group.Steps {
			step.Description = escapeText(step.Description, s)
			step.Error = escapeText(step.Error, s)
			for i := range step.OutputLines {
				step.OutputLines[i] = escapeText(step.OutputLines[i], s)
			}
		}
	}
}
