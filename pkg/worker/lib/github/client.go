package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/cenkalti/backoff"
	gh "github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
)

//go:generate mockgen -package github -source client.go -destination client_mock.go

type Status string

var (
	ErrPRNotFound            = errors.New("no such pull request")
	ErrUnauthorized          = errors.New("invalid authorization")
	ErrUserIsBlocked         = errors.New("user is blocked")
	ErrCommitIsNotPartOfPull = errors.New("commit is not part of the pull request")
)

func IsRecoverableError(err error) bool {
	err = errors.Cause(err)
	return err != ErrPRNotFound && err != ErrUnauthorized &&
		err != ErrUserIsBlocked && err != ErrCommitIsNotPartOfPull
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
		if er.Response.StatusCode == http.StatusUnprocessableEntity {
			if strings.Contains(er.Error(), "User is blocked") {
				return ErrUserIsBlocked
			}
			if strings.Contains(er.Error(), "Commit is not part of the pull request") {
				return ErrCommitIsNotPartOfPull
			}
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
	// TODO: migrate to common provider client from api

	// don't use c.GetClient(ctx).PullRequests.CreateReview
	// because of https://github.com/google/go-github/issues/540

	bodyReader := &bytes.Buffer{}
	enc := json.NewEncoder(bodyReader)
	enc.SetEscapeHTML(false)
	err := enc.Encode(review)
	if err != nil {
		return errors.Wrap(err, "failed to json encode review")
	}

	u := fmt.Sprintf("https://api.github.com/repos/%v/%v/pulls/%d/reviews", c.Repo.Owner, c.Repo.Name, c.PullRequestNumber)
	req, err := http.NewRequest(http.MethodPost, u, bodyReader)
	if err != nil {
		return errors.Wrap(err, "failed to make new http request")
	}
	req.Header.Set("Content-Type", "application/json")
	const mediaTypeV3 = "application/vnd.github.v3+json"
	req.Header.Set("Accept", mediaTypeV3)

	resp, err := c.GetHTTPClient(ctx).Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to perform http request")
	}

	if resp.Body == nil {
		return errors.Wrap(err, "no response body")
	}

	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	ge := &gh.ErrorResponse{
		Response: resp,
		Message:  string(respBody),
	}
	if terr := transformGithubError(ge); terr != nil {
		return terr
	}

	return errors.Wrap(ge, "can't create github review")
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
		Context:     gh.String(os.Getenv("APP_NAME")),
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
