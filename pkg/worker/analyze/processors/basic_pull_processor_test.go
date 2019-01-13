package processors_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/golangci/golangci-api/pkg/worker/analytics"

	"github.com/golangci/golangci-api/pkg/worker/analyze/processors"

	"github.com/golangci/golangci-api/internal/shared/logutil"

	"github.com/golang/mock/gomock"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters"
	"github.com/golangci/golangci-api/pkg/worker/analyze/linters/result"
	"github.com/golangci/golangci-api/pkg/worker/analyze/prstate"
	"github.com/golangci/golangci-api/pkg/worker/analyze/reporters"
	"github.com/golangci/golangci-api/pkg/worker/lib/executors"
	"github.com/golangci/golangci-api/pkg/worker/lib/fetchers"
	"github.com/golangci/golangci-api/pkg/worker/lib/github"
	"github.com/golangci/golangci-api/pkg/worker/test"
	gh "github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
)

var testCtx = analytics.ContextWithEventPropsCollector(context.Background(), analytics.EventPRChecked)
var testCtxMatcher = gomock.Any()

var any = gomock.Any()
var fakeChangedIssue = result.NewIssue("linter2", "F1 issue", "main.go", 10, 11)
var fakeChangedIssues = []result.Issue{
	result.NewIssue("linter2", "F1 issue", "main.go", 9, 10),
	result.NewIssue("linter3", "F1 issue", "main.go", 10, 11),
}

var testSHA = "testSHA"
var testBranch = "testBranch"
var testPR = &gh.PullRequest{
	Head: &gh.PullRequestBranch{
		Ref: gh.String(testBranch),
		SHA: gh.String(testSHA),
	},
	Base: &gh.PullRequestBranch{
		Repo: &gh.Repository{
			Private: gh.Bool(false),
		},
	},
	Number: gh.Int(7),
}
var testAnalysisGUID = "test-guid"
var testUserID = 1

var testProviderCtx = &github.Context{
	Repo: github.Repo{
		Owner: "owner",
		Name:  "name",
	},
	GithubAccessToken: "access_token",
	PullRequestNumber: 1,
}

func getRealisticTestProviderCtx(t *testing.T) *github.Context {
	repo := os.Getenv("REPO")
	if repo == "" {
		repo = "golangci/golangci-api"
	}

	repoParts := strings.Split(repo, "/")
	assert.Len(t, repoParts, 2)

	pr := os.Getenv("PR")
	if pr == "" {
		pr = "1"
	}
	prNumber, err := strconv.Atoi(pr)
	assert.NoError(t, err)

	c := &github.Context{
		Repo: github.Repo{
			Owner: repoParts[0],
			Name:  repoParts[1],
		},
		PullRequestNumber: prNumber,
		GithubAccessToken: os.Getenv("TEST_GITHUB_TOKEN"),
	}

	return c
}

func getTestPullCtx() *processors.PullContext {
	return &processors.PullContext{
		Ctx:          testCtx,
		UserID:       testUserID,
		AnalysisGUID: testAnalysisGUID,
		ProviderCtx:  testProviderCtx,
		LogCtx:       logutil.Context{},
		Log:          logutil.NewStderrLog("test"),
	}
}

func getRealisticTestPullCtx(t *testing.T) *processors.PullContext {
	pullCtx := getTestPullCtx()
	pullCtx.ProviderCtx = getRealisticTestProviderCtx(t)
	return pullCtx
}

func getFakeLinters(ctrl *gomock.Controller, issues ...result.Issue) []linters.Linter {
	a := linters.NewMockLinter(ctrl)
	a.EXPECT().
		Run(testCtxMatcher, any, any).
		Return(&result.Result{
			Issues: issues,
		}, nil)
	return []linters.Linter{a}
}

func getNopFetcher(ctrl *gomock.Controller) fetchers.Fetcher {
	f := fetchers.NewMockFetcher(ctrl)
	f.EXPECT().Fetch(testCtxMatcher, any, any, any).Return(nil)
	return f
}

func getNopReporter(ctrl *gomock.Controller) reporters.Reporter {
	r := reporters.NewMockReporter(ctrl)
	r.EXPECT().Report(testCtxMatcher, any, any, any).AnyTimes().Return(nil)
	return r
}

func getErroredReporter(ctrl *gomock.Controller) reporters.Reporter {
	r := reporters.NewMockReporter(ctrl)
	r.EXPECT().Report(testCtxMatcher, any, any, any).Return(fmt.Errorf("can't report"))
	return r
}

func getNopState(ctrl *gomock.Controller) prstate.Storage {
	r := prstate.NewMockStorage(ctrl)
	r.EXPECT().UpdateState(any, any, any, any, any).AnyTimes().Return(nil)
	r.EXPECT().GetState(any, any, any, any).AnyTimes().Return(&prstate.State{
		Status: processors.StatusSentToQueue,
	}, nil)
	return r
}

func getNopExecutor(ctrl *gomock.Controller) executors.Executor {
	e := executors.NewMockExecutor(ctrl)
	e.EXPECT().WorkDir().Return("").AnyTimes()
	e.EXPECT().WithWorkDir(any).Return(e).AnyTimes()
	runRes := &executors.RunResult{StdOut: "{}"}
	e.EXPECT().Run(testCtxMatcher, "goenvbuild").Return(runRes, nil).AnyTimes()
	e.EXPECT().Run(testCtxMatcher, "golangci-lint", any).Return(runRes, nil).AnyTimes()
	e.EXPECT().Clean().AnyTimes()
	e.EXPECT().WithEnv(any, any).Return(e).AnyTimes()
	e.EXPECT().SetEnv(any, any).AnyTimes()
	e.EXPECT().CopyFile(any, any, any).Return(nil)
	return e
}

func getFakePatch(t *testing.T) string {
	patch, err := ioutil.ReadFile(fmt.Sprintf("test/%d.patch", testProviderCtx.PullRequestNumber))
	assert.Nil(t, err)
	return string(patch)
}

func getFakeStatusGithubClient(t *testing.T, ctrl *gomock.Controller, status github.Status, statusDesc string) github.Client {
	c := testProviderCtx
	gc := github.NewMockClient(ctrl)
	gc.EXPECT().GetPullRequest(testCtxMatcher, c).Return(testPR, nil)

	scsPending := gc.EXPECT().SetCommitStatus(testCtxMatcher, c, testSHA,
		github.StatusPending, "GolangCI is reviewing your Pull Request...", "").
		Return(nil)

	gc.EXPECT().GetPullRequestPatch(any, any).AnyTimes().Return(getFakePatch(t), nil)

	test.Init()
	url := fmt.Sprintf("%s/r/github.com/%s/%s/pulls/%d", os.Getenv("WEB_ROOT"), c.Repo.Owner, c.Repo.Name, testPR.GetNumber())
	gc.EXPECT().SetCommitStatus(testCtxMatcher, c, testSHA, status, statusDesc, url).After(scsPending)

	return gc
}

func getNopGithubClient(t *testing.T, ctrl *gomock.Controller) github.Client {
	c := &testProviderCtx

	gc := github.NewMockClient(ctrl)
	gc.EXPECT().CreateReview(any, any, any).AnyTimes()
	gc.EXPECT().GetPullRequest(testCtxMatcher, c).AnyTimes().Return(testPR, nil)
	gc.EXPECT().GetPullRequestPatch(any, any).AnyTimes().Return(getFakePatch(t))
	gc.EXPECT().SetCommitStatus(any, any, testSHA, any, any, any).AnyTimes()
	return gc
}

func fillWithNops(t *testing.T, ctrl *gomock.Controller, cfg *processors.BasicPullConfig) {
	if cfg.ProviderClient == nil {
		cfg.ProviderClient = getNopGithubClient(t, ctrl)
	}
	if cfg.Exec == nil {
		cfg.Exec = getNopExecutor(ctrl)
	}
	if cfg.Linters == nil {
		cfg.Linters = getFakeLinters(ctrl)
	}
	if cfg.RepoFetcher == nil {
		cfg.RepoFetcher = getNopFetcher(ctrl)
	}
	if cfg.Reporter == nil {
		cfg.Reporter = getNopReporter(ctrl)
	}
	if cfg.State == nil {
		cfg.State = getNopState(ctrl)
	}
}

func getNopedProcessor(t *testing.T, ctrl *gomock.Controller, cfg *processors.BasicPullConfig) processors.PullProcessor {
	fillWithNops(t, ctrl, cfg)

	pf := processors.NewBasicPullProcessorFactory(cfg)
	pullCtx := getTestPullCtx()
	p, _, err := pf.BuildProcessor(pullCtx)
	assert.NoError(t, err)

	return p
}

func testProcessor(t *testing.T, ctrl *gomock.Controller, cfg *processors.BasicPullConfig) {
	p := getNopedProcessor(t, ctrl, cfg)

	err := p.Process(getTestPullCtx())
	assert.NoError(t, err)
}

func TestSetCommitStatusSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testProcessor(t, ctrl, &processors.BasicPullConfig{
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			Linters:        getFakeLinters(ctrl),
			ProviderClient: getFakeStatusGithubClient(t, ctrl, github.StatusSuccess, "No issues found!"),
		},
	})
}

func TestSetCommitStatusFailureOneIssue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testProcessor(t, ctrl, &processors.BasicPullConfig{
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			Linters:        getFakeLinters(ctrl, fakeChangedIssue),
			ProviderClient: getFakeStatusGithubClient(t, ctrl, github.StatusFailure, "1 issue found"),
		},
	})
}

func TestSetCommitStatusFailureTwoIssues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testProcessor(t, ctrl, &processors.BasicPullConfig{
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			Linters:        getFakeLinters(ctrl, fakeChangedIssues...),
			ProviderClient: getFakeStatusGithubClient(t, ctrl, github.StatusFailure, "2 issues found"),
		},
	})
}

func TestSetCommitStatusOnReportingError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p := getNopedProcessor(t, ctrl, &processors.BasicPullConfig{
		Reporter: getErroredReporter(ctrl),
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			Linters: getFakeLinters(ctrl, fakeChangedIssue),
			ProviderClient: getFakeStatusGithubClient(t, ctrl,
				github.StatusError, "can't send pull request comments to github"),
		},
	})

	err := p.Process(getTestPullCtx())
	t.Logf("Got expected error: %s", err)
	assert.Error(t, err)
}

//nolint
func getRealisticTestProcessor(pullCtx *processors.PullContext, t *testing.T, ctrl *gomock.Controller) processors.PullProcessor {
	c := pullCtx.ProviderCtx
	cloneURL := fmt.Sprintf("git@github.com:%s/%s.git", c.Repo.Owner, c.Repo.Name)
	pr := &gh.PullRequest{
		Head: &gh.PullRequestBranch{
			Ref: gh.String("master"),
			Repo: &gh.Repository{
				SSHURL: gh.String(cloneURL),
			},
		},
	}
	gc := github.NewMockClient(ctrl)
	gc.EXPECT().GetPullRequest(testCtxMatcher, c).Return(pr, nil).AnyTimes()
	gc.EXPECT().GetPullRequestPatch(any, any).AnyTimes().Return(getFakePatch(t), nil)
	gc.EXPECT().SetCommitStatus(any, any, any, any, any, any).AnyTimes()

	exec, err := executors.NewTempDirShell("gopath")
	assert.NoError(t, err)

	pf := processors.NewBasicPullProcessorFactory(&processors.BasicPullConfig{
		Reporter: getNopReporter(ctrl),
		Exec:     exec,
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			ProviderClient: gc,
		},
	})

	p, _, err := pf.BuildProcessor(pullCtx)
	assert.NoError(t, err)

	return p
}

func getTestProcessorWithFakeGithub(pullCtx *processors.PullContext, t *testing.T, ctrl *gomock.Controller) processors.PullProcessor {
	c := pullCtx.ProviderCtx

	realGc := github.NewMyClient()
	patch, err := realGc.GetPullRequestPatch(pullCtx.Ctx, c)
	assert.NoError(t, err)

	pr, err := realGc.GetPullRequest(pullCtx.Ctx, c)
	assert.NoError(t, err)

	gc := github.NewMockClient(ctrl)
	gc.EXPECT().GetPullRequestPatch(any, any).AnyTimes().Return(patch, nil)
	gc.EXPECT().GetPullRequest(testCtxMatcher, c).Return(pr, nil)
	gc.EXPECT().SetCommitStatus(any, any, any, any, any, any).AnyTimes()

	pf := processors.NewBasicPullProcessorFactory(&processors.BasicPullConfig{
		Reporter: getNopReporter(ctrl),
		StaticBasicPullConfig: processors.StaticBasicPullConfig{
			ProviderClient: gc,
		},
	})

	p, _, err := pf.BuildProcessor(pullCtx)
	assert.NoError(t, err)

	return p
}

func TestProcessRepoWithFakeGithub(t *testing.T) {
	test.Init()
	test.MarkAsSlow(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pullCtx := getRealisticTestPullCtx(t)
	p := getTestProcessorWithFakeGithub(pullCtx, t, ctrl)
	err := p.Process(pullCtx)
	assert.NoError(t, err)
}
