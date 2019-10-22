package score

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/golangci/golangci-lint/pkg/printers"
)

type Calculator struct{}

type Recommendation struct {
	Text          string
	ScoreIncrease int // [0; 100], how much score can be gained if perform recommendation
}

type CalcResult struct {
	Score           int // [0; 100]
	MaxScore        int
	Recommendations []Recommendation
}

type weightedLinter struct {
	name   string  // linter name
	weight float64 // importance of linter
}

func (c Calculator) Calc(runRes *printers.JSONResult) *CalcResult {
	const maxScore = 100

	if runRes.Report == nil {
		return &CalcResult{
			Score:    maxScore,
			MaxScore: maxScore,
		}
	}

	var recomendations []Recommendation
	if rec := c.buildRecommendationForDisabledLinters(runRes); rec != nil {
		recomendations = append(recomendations, *rec)
	}

	if rec := c.buildRecommendationForIssues(runRes); rec != nil {
		recomendations = append(recomendations, *rec)
	}

	score := maxScore
	for _, rec := range recomendations {
		score -= rec.ScoreIncrease
	}

	return &CalcResult{
		Score:           score,
		MaxScore:        maxScore,
		Recommendations: recomendations,
	}
}

func (c Calculator) buildRecommendationForDisabledLinters(runRes *printers.JSONResult) *Recommendation {
	enabledLinters := map[string]bool{}
	for _, linter := range runRes.Report.Linters {
		if linter.Enabled {
			enabledLinters[linter.Name] = true
		}
	}

	linters := c.getNeededLinters(enabledLinters)

	var disabledNeededLinters []weightedLinter
	for _, wl := range linters {
		if !enabledLinters[wl.name] {
			disabledNeededLinters = append(disabledNeededLinters, wl)
		}
	}

	if len(disabledNeededLinters) == 0 {
		return nil
	}

	weight := float64(0)
	var disabledNeededLinterNames []string
	for _, wl := range disabledNeededLinters {
		weight += wl.weight
		disabledNeededLinterNames = append(disabledNeededLinterNames, wl.name)
	}

	sort.Strings(disabledNeededLinterNames)

	const maxScore = 100
	score := int(weight * maxScore)
	if score == 0 { // rounded to zero
		return nil
	}

	return &Recommendation{
		ScoreIncrease: score,
		Text:          fmt.Sprintf("enable linters %s", strings.Join(disabledNeededLinterNames, ", ")),
	}
}

//nolint:gocyclo
func (c Calculator) buildRecommendationForIssues(runRes *printers.JSONResult) *Recommendation {
	enabledLinters := map[string]bool{}
	for _, linter := range runRes.Report.Linters {
		if linter.Enabled {
			enabledLinters[linter.Name] = true
		}
	}

	linters := c.getNeededLinters(enabledLinters)

	lintersMap := map[string]*weightedLinter{}
	for i := range linters {
		lintersMap[linters[i].name] = &linters[i]
	}

	issuesPerLinter := map[string]int{}
	for _, issue := range runRes.Issues {
		issuesPerLinter[issue.FromLinter]++
	}

	if len(issuesPerLinter) == 0 {
		return nil
	}

	weight := float64(0)
	for linter, issueCount := range issuesPerLinter {
		wl := lintersMap[linter]
		if wl == nil {
			continue // not needed linter
		}

		if issueCount > 100 {
			issueCount = 100
		}

		// 100 -> 1, 50 -> 0.85, 10 -> 0.5, 5 -> 0.35, 1 -> 0
		normalizedLog := math.Log10(float64(issueCount)) / 2
		const minScoreForAnyIssue = 0.2
		weight += wl.weight * (minScoreForAnyIssue + (1-minScoreForAnyIssue)*normalizedLog)
	}

	var neededLintersWithIssues []string
	for linter := range issuesPerLinter {
		if _, ok := lintersMap[linter]; ok {
			neededLintersWithIssues = append(neededLintersWithIssues, linter)
		}
	}

	sort.Strings(neededLintersWithIssues)

	const maxScore = 100
	score := int(weight * maxScore)
	if score == 0 { // rounded to zero
		return nil
	}

	return &Recommendation{
		ScoreIncrease: score,
		Text:          fmt.Sprintf("fix issues from linters %s", strings.Join(neededLintersWithIssues, ", ")),
	}
}

func (c Calculator) getNeededLinters(enabledLinters map[string]bool) []weightedLinter {
	bugsLinters := c.getNeededBugsLintersWeights()
	styleLinters := c.getNeededStyleLintersWeights(enabledLinters)

	const bugsWeight = 0.7
	var linters []weightedLinter
	for _, wl := range bugsLinters {
		wl.weight *= bugsWeight
		linters = append(linters, wl)
	}
	for _, wl := range styleLinters {
		wl.weight *= 1 - bugsWeight
		linters = append(linters, wl)
	}

	return linters
}

func (c Calculator) normalizeWeightedLinters(linters []weightedLinter) []weightedLinter {
	res := make([]weightedLinter, 0, len(linters))
	totalWeight := float64(0)
	for _, wl := range linters {
		totalWeight += wl.weight
	}

	for _, wl := range linters {
		res = append(res, weightedLinter{wl.name, wl.weight / totalWeight})
	}

	return res
}

func (c Calculator) getNeededBugsLintersWeights() []weightedLinter {
	return c.normalizeWeightedLinters([]weightedLinter{
		{"govet", 1},
		{"staticcheck", 1},
		{"errcheck", 0.8},
		{"bodyclose", 0.7}, // low because can have false-positives
		{"typecheck", 0.5},
	})
}

func (c Calculator) getNeededStyleLintersWeights(enabledLinters map[string]bool) []weightedLinter {
	linters := []weightedLinter{
		{"goimports", 1},
		{"dogsled", 0.5},
		{"gochecknoglobals", 0.4}, // low because can have false-positives
		{"gochecknoinits", 0.4},
		{"goconst", 0.3},
		{"golint", 1},
		{"gosimple", 0.6},
		{"lll", 0.1},
		{"misspell", 0.4},
		{"unconvert", 0.4},
		{"ineffassign", 0.5},
	}

	const (
		gocognit = "gocognit"
		gocyclo  = "gocyclo"
	)
	complexityLinter := gocognit
	if !enabledLinters[gocognit] && enabledLinters[gocyclo] {
		complexityLinter = gocyclo
	}
	linters = append(linters, weightedLinter{complexityLinter, 0.8})

	return c.normalizeWeightedLinters(linters)
}
