package repos

import (
	"fmt"
	"strings"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/providers/implementations"
)

func getReviewerLogin(providerName string, cfg config.Config) string {
	if providerName == implementations.GithubProviderName {
		providerName = "github" // TODO: rename config var
	}
	key := fmt.Sprintf("%s_REVIEWER_LOGIN", strings.ToUpper(providerName))
	return cfg.GetString(key)
}
