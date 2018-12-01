package experiments

import (
	"hash/fnv"
	"strings"

	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/logutil"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
)

type Checker struct {
	cfg config.Config
	log logutil.Log
}

func NewChecker(cfg config.Config, log logutil.Log) *Checker {
	return &Checker{cfg: cfg, log: log}
}

func (c Checker) getConfigKey(name, suffix string) string {
	return strings.ToUpper(name + "_" + suffix)
}

func (c Checker) parseConfigVarToBoolMap(k string) map[string]bool {
	elems := c.cfg.GetString(k)
	if elems == "" {
		return map[string]bool{}
	}

	elemList := strings.Split(elems, ",")
	ret := map[string]bool{}
	for _, e := range elemList {
		ret[e] = true
	}

	return ret
}

func (c Checker) IsActiveForAnalysis(name string, repo *github.Repo, forPull bool) bool {
	if forPull && !c.cfg.GetBool(c.getConfigKey(name, "for_pulls"), false) {
		c.log.Infof("Experiment %s is disabled for pull analyzes", name)
		return false
	}

	enabledRepos := c.parseConfigVarToBoolMap(c.getConfigKey(name, "repos"))
	if enabledRepos[repo.FullName()] {
		c.log.Infof("Experiment %s is enabled for repo %s", name, repo.FullName())
		return true
	}

	enabledOwners := c.parseConfigVarToBoolMap(c.getConfigKey(name, "owners"))
	if enabledOwners[repo.Owner] {
		c.log.Infof("Experiment %s is enabled for owner of repo %s", name, repo.FullName())
		return true
	}

	percent := c.cfg.GetInt(c.getConfigKey(name, "percent"), 0)
	if percent < 0 || percent > 100 {
		c.log.Infof("Experiment %s is disabled: invalid percent %d", name, percent)
		return false
	}

	hash := hash(repo.FullName())
	if uint32(percent) <= (hash % 100) {
		c.log.Infof("Experiment %s is disabled by percent for repo %s: %d (percent) <= %d (hash mod 100)",
			name, repo.FullName(), percent, hash%100)
		return false
	}

	c.log.Infof("Experiment %s is enabled by percent for repo %s: %d (percent) > %d (hash mod 100)",
		name, repo.FullName(), percent, hash%100)
	return true
}

func hash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
