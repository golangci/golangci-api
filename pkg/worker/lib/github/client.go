package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	gh "github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package github -source client.go -destination client_mock.go

type Status string

var ErrPRNotFound = errors.New("no such pull request")
var ErrUnauthorized = errors.New("invalid authorization")

func IsRecoverableError(err error) bool {
	return err != ErrPRNotFound && err != ErrUnauthorized
}

const (
	StatusPending Status = "pending"
	StatusFailure Status = "failure"
	StatusError   Status = "error"
	StatusSuccess Status = "success"
)

type Client interface {
	GetPullRequest(ctx context.Context, c *Context) (*gh.PullRequest, error)
	GetPullRequestComments(ctx context.Context, c *Context) ([]*gh.PullRequestComment, error)
	GetPullRequestPatch(ctx context.Context, c *Context) (string, error)
	CreateReview(ctx context.Context, c *Context, review *gh.PullRequestReviewRequest) error
	SetCommitStatus(ctx context.Context, c *Context, ref string, status Status, desc, url string) error
}

type MyClient struct{}

var _ Client = &MyClient{}

func NewMyClient() *MyClient {
	return &MyClient{}
}

func transformGithubError(err error) error {
	if er, ok := err.(*gh.ErrorResponse); ok {
		if er.Response.StatusCode == http.StatusNotFound {
			logrus.Warnf("Got 404 from github: %+v", er)
			return ErrPRNotFound
		}
		if er.Response.StatusCode == http.StatusUnauthorized {
			logrus.Warnf("Got 401 from github: %+v", er)
			return ErrUnauthorized
		}
	}

	return nil
}

func retryGet(f func() error) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 2 * time.Minute

	bmr := backoff.WithMaxRetries(b, 5)

	if err := backoff.Retry(f, bmr); err != nil {
		logrus.Warnf("Github operation failed to retry with %v and took %s: %s", b, b.GetElapsedTime(), err)
		return err
	}

	return nil
}

func (gc *MyClient) GetPullRequest(ctx context.Context, c *Context) (*gh.PullRequest, error) {
	var retPR *gh.PullRequest

	f := func() error {
		pr, _, err := c.GetClient(ctx).PullRequests.Get(ctx, c.Repo.Owner, c.Repo.Name, c.PullRequestNumber)
		if err != nil {
			return err
		}

		retPR = pr
		return nil
	}

	if err := retryGet(f); err != nil {
		if terr := transformGithubError(err); terr != nil {
			return nil, terr
		}

		return nil, fmt.Errorf("can't get pull request %d from github: %s", c.PullRequestNumber, err)
	}

	return retPR, nil
}

func (gc *MyClient) CreateReview(ctx context.Context, c *Context, review *gh.PullRequestReviewRequest) error {
	_, _, err := c.GetClient(ctx).PullRequests.CreateReview(ctx, c.Repo.Owner, c.Repo.Name, c.PullRequestNumber, review)
	if err != nil {
		if terr := transformGithubError(err); terr != nil {
			return terr
		}

		return fmt.Errorf("can't create github review: %s", err)
	}

	return nil
}

func (gc *MyClient) GetPullRequestPatch(ctx context.Context, c *Context) (string, error) {
	var ret string

	f := func() error {
		opts := gh.RawOptions{Type: gh.Diff}
		raw, _, err := c.GetClient(ctx).PullRequests.GetRaw(ctx, c.Repo.Owner, c.Repo.Name,
			c.PullRequestNumber, opts)
		if err != nil {
			return err
		}

		ret = raw
		return nil
	}

	if err := retryGet(f); err != nil {
		if terr := transformGithubError(err); terr != nil {
			return "", terr
		}

		return "", fmt.Errorf("can't get patch for pull request: %s", err)
	}

	return ret, nil
}

func (gc *MyClient) SetCommitStatus(ctx context.Context, c *Context, ref string, status Status, desc, url string) error {
	rs := &gh.RepoStatus{
		Description: gh.String(desc),
		State:       gh.String(string(status)),
		Context:     gh.String("GolangCI"),
	}
	if url != "" {
		rs.TargetURL = gh.String(url)
	}
	_, _, err := c.GetClient(ctx).Repositories.CreateStatus(ctx, c.Repo.Owner, c.Repo.Name, ref, rs)
	if err != nil {
		if terr := transformGithubError(err); terr != nil {
			return terr
		}

		return fmt.Errorf("can't set commit %s status %s: %s", ref, status, err)
	}

	return nil
}

func (gc *MyClient) GetPullRequestComments(ctx context.Context, c *Context) ([]*gh.PullRequestComment, error) {
	var ret []*gh.PullRequestComment

	f := func() error {
		opt := &gh.PullRequestListCommentsOptions{
			ListOptions: gh.ListOptions{
				PerPage: 100, // max allowed value, TODO: fetch all comments if >100
			},
		}
		comments, _, err := c.GetClient(ctx).PullRequests.ListComments(ctx, c.Repo.Owner, c.Repo.Name, c.PullRequestNumber, opt)
		if err != nil {
			return err
		}

		ret = comments
		return nil
	}

	if err := retryGet(f); err != nil {
		if terr := transformGithubError(err); terr != nil {
			return nil, terr
		}

		return nil, fmt.Errorf("can't get pull request %d comments from github: %s", c.PullRequestNumber, err)
	}

	return ret, nil
}
