package reporters

import (
	"context"
	"fmt"
	"os"

	"github.com/golangci/golangci-api/pkg/worker/analytics"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	gh "github.com/google/go-github/github"
)

type GithubReviewer struct {
	*github.Context
	client            github.Client
	includeLinterName bool
}

func NewGithubReviewer(c *github.Context, client github.Client, includeLinterName bool) *GithubReviewer {
	accessToken := os.Getenv("GITHUB_REVIEWER_ACCESS_TOKEN")
	if accessToken != "" { // review as special user
		cCopy := *c
		cCopy.GithubAccessToken = accessToken
		c = &cCopy
	}
	ret := &GithubReviewer{
		Context:           c,
		client:            client,
		includeLinterName: includeLinterName,
	}
	return ret
}

type existingComment struct {
	file string
	line int
}

type existingComments []existingComment

func (ecs existingComments) contains(i *result.Issue) bool {
	for _, c := range ecs {
		if c.file == i.File && c.line == i.HunkPos {
			return true
		}
	}

	return false
}

func (gr GithubReviewer) fetchExistingComments(ctx context.Context) (existingComments, error) {
	comments, err := gr.client.GetPullRequestComments(ctx, gr.Context)
	if err != nil {
		return nil, err
	}

	var ret existingComments
	for _, c := range comments {
		if c.Position == nil { // comment on outdated code, skip it
			continue
		}
		ret = append(ret, existingComment{
			file: c.GetPath(),
			line: c.GetPosition(),
		})
	}

	return ret, nil
}

func (gr GithubReviewer) Report(ctx context.Context, ref string, issues []result.Issue) error {
	if len(issues) == 0 {
		analytics.Log(ctx).Infof("Nothing to report")
		return nil
	}

	existingComments, err := gr.fetchExistingComments(ctx)
	if err != nil {
		return err
	}

	comments := []*gh.DraftReviewComment{}
	for _, i := range issues {
		if existingComments.contains(&i) {
			continue // don't be annoying: don't comment on the same line twice
		}

		text := i.Text
		if gr.includeLinterName && i.FromLinter != "" {
			text += fmt.Sprintf(" (from `%s`)", i.FromLinter)
		}

		comment := &gh.DraftReviewComment{
			Path:     gh.String(i.File),
			Position: gh.Int(i.HunkPos),
			Body:     gh.String(text),
		}
		comments = append(comments, comment)
	}

	if len(comments) == 0 {
		return nil // all comments are already exist
	}

	review := &gh.PullRequestReviewRequest{
		CommitID: gh.String(ref),
		Event:    gh.String("COMMENT"),
		Body:     gh.String(""),
		Comments: comments,
	}
	if err := gr.client.CreateReview(ctx, gr.Context, review); err != nil {
		return fmt.Errorf("can't create review %+v: %s", review, err)
	}

	analytics.Log(ctx).Infof("Submitted review %+v, existing comments: %+v, issues: %+v",
		review, existingComments, issues)
	return nil
}
