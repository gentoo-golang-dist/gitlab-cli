//go:build !integration

package visualize

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), "testdata", name)
}


type recordedLintRequest struct {
	Content     string `json:"content"`
	DryRun      bool   `json:"dry_run"`
	IncludeJobs bool   `json:"include_jobs"`
	Ref         string `json:"ref"`
}


type lintStub struct {
	t              *testing.T
	mu             sync.Mutex
	lastLintReq    recordedLintRequest
	runnableJobs   []map[string]string // jobs to return when include_jobs=true
	forceLintError bool
}

func newLintStub(t *testing.T) *lintStub {
	t.Helper()
	return &lintStub{t: t}
}

func (s *lintStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/OWNER/REPO"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"path":"REPO","path_with_namespace":"OWNER/REPO"}`))

		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/1/ci/lint"):
			body, _ := io.ReadAll(r.Body)
			var req recordedLintRequest
			_ = json.Unmarshal(body, &req)

			s.mu.Lock()
			s.lastLintReq = req
			runnable := s.runnableJobs
			forceErr := s.forceLintError
			s.mu.Unlock()

			if forceErr {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"valid":false,"errors":["boom"],"merged_yaml":""}`))
				return
			}

			resp := map[string]any{
				"valid":       true,
				"errors":      []string{},
				"warnings":    []string{},
				"merged_yaml": req.Content,
				"includes":    []any{},
			}
			if req.IncludeJobs {
				jobs := runnable
				if jobs == nil {
					// Default: echo every top-level mapping key that looks like a job.
					jobs = deriveJobsFromYAML(req.Content)
				}
				resp["jobs"] = jobs
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		default:
			s.t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

// deriveJobsFromYAML is used to pull top-level job names
func deriveJobsFromYAML(content string) []map[string]string {
	var out []map[string]string
	lines := strings.Split(content, "\n")
	inBlock := false
	for _, line := range lines {
		if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		if strings.HasPrefix(line, "---") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 1 {
			continue
		}
		key := line[:colon]
		rest := strings.TrimSpace(line[colon+1:])
		// Scalars like "stages: [...]" are not jobs.
		if rest != "" {
			inBlock = false
			continue
		}
		inBlock = true
		switch key {
		case "stages", "variables", "include", "default", "workflow",
			"image", "services", "before_script", "after_script", "cache":
			continue
		}
		if strings.HasPrefix(key, ".") {
			continue
		}
		out = append(out, map[string]string{"name": key, "stage": "test"})
	}
	_ = inBlock
	return out
}

func (s *lintStub) setRunnableJobs(jobs []map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runnableJobs = jobs
}

func (s *lintStub) setForceLintError(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forceLintError = v
}

func (s *lintStub) lastRequest() recordedLintRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastLintReq
}

// setupVisualizeTest wires a cobra command backed by a live test HTTP server
func setupVisualizeTest(t *testing.T, extra ...cmdtest.FactoryOption) (cmdtest.CmdExecFunc, *lintStub) {
	t.Helper()

	stub := newLintStub(t)
	server := httptest.NewServer(stub.handler())
	t.Cleanup(server.Close)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL+"/api/v4/"))
	require.NoError(t, err)

	opts := append([]cmdtest.FactoryOption{cmdtest.WithGitLabClient(client)}, extra...)
	exec := cmdtest.SetupCmdForTest(t, NewCmdVisualize, false, opts...)
	return exec, stub
}

func TestNewCmdVisualize_InvalidPath(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	_, err := exec("NONEXISTENT_FILE")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestNewCmdVisualize_EmptyPipeline(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	_, err := exec(testdataPath("empty.yml"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no jobs found")
}

func TestNewCmdVisualize_SVGOutput(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	result, err := exec(testdataPath("simple.yml") + " --output svg")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
	assert.Contains(t, result.String(), "</svg>")
}

func TestNewCmdVisualize_SVGOutputWithNeeds(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	result, err := exec(testdataPath("needs.yml") + " --output svg")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
}

func TestNewCmdVisualize_InvalidOutputFlag(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output mode")
}

func TestNewCmdVisualize_OutputSVGConflictsWithWeb(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output svg --web")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output svg cannot be combined with --web")
}

func TestNewCmdVisualize_NoStageEdges(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t)

	result, err := exec(testdataPath("simple.yml") + " --output svg --stage-edges=false")

	require.NoError(t, err)
	assert.Contains(t, result.String(), "<svg")
}

// Simulation flags should cause the command to inject variables 
func TestNewCmdVisualize_BranchSimulation_InjectsCIVariables(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output svg --branch main")
	require.NoError(t, err)

	req := stub.lastRequest()
	assert.True(t, req.DryRun, "simulation flags must trigger dry_run")
	assert.True(t, req.IncludeJobs, "simulation flags must trigger include_jobs")
	assert.Contains(t, req.Content, "CI_COMMIT_BRANCH: main")
	assert.Contains(t, req.Content, "CI_PIPELINE_SOURCE: push")
}

func TestNewCmdVisualize_MRSimulation_InjectsCIVariables(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output svg --source-branch feat/x --target-branch main")
	require.NoError(t, err)

	req := stub.lastRequest()
	assert.True(t, req.DryRun)
	assert.Contains(t, req.Content, "CI_PIPELINE_SOURCE: merge_request_event")
	assert.Contains(t, req.Content, "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME: feat/x")
	assert.Contains(t, req.Content, "CI_MERGE_REQUEST_TARGET_BRANCH_NAME: main")
}

func TestNewCmdVisualize_CustomVarInjection(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output svg --var DEPLOY_ENV=staging")
	require.NoError(t, err)

	req := stub.lastRequest()
	assert.True(t, req.DryRun)
	assert.Contains(t, req.Content, "DEPLOY_ENV: staging")
}

func TestNewCmdVisualize_APIFilterPrunesJobs(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)
	// API claims only "compile" should run; "unit-tests" and others should
	// be pruned from the rendered DAG even though they're in merged_yaml.
	// Need to follow up on this later
	stub.setRunnableJobs([]map[string]string{
		{"name": "compile", "stage": "build"},
	})

	result, err := exec(testdataPath("simple.yml") + " --output svg --branch main")
	require.NoError(t, err)

	svg := result.String()
	assert.Contains(t, svg, "compile")
	assert.NotContains(t, svg, "unit-tests")
	assert.NotContains(t, svg, "deploy-prod")
}

func TestNewCmdVisualize_APIFilterReportsNoRunnableJobs(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)
	stub.setRunnableJobs([]map[string]string{}) // nothing would run

	_, err := exec(testdataPath("simple.yml") + " --output svg --branch main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no jobs would run")
}

func TestNewCmdVisualize_LintAPIReturnsInvalid(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)
	stub.setForceLintError(true)

	_, err := exec(testdataPath("simple.yml") + " --output svg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CI/CD configuration is invalid")
}

func TestNewCmdVisualize_NoBaseRepo(t *testing.T) {
	t.Parallel()

	exec, _ := setupVisualizeTest(t, cmdtest.WithBaseRepoError(errors.New("no base repository present")))

	_, err := exec(testdataPath("simple.yml") + " --output svg")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ci visualize needs a GitLab project")
}

func TestNewCmdVisualize_DoesNotSimulateWithoutFlags(t *testing.T) {
	t.Parallel()

	exec, stub := setupVisualizeTest(t)

	_, err := exec(testdataPath("simple.yml") + " --output svg")
	require.NoError(t, err)

	req := stub.lastRequest()
	assert.False(t, req.DryRun, "plain invocation should not set dry_run")
	assert.False(t, req.IncludeJobs, "plain invocation should not request jobs")
	assert.NotContains(t, req.Content, "CI_PIPELINE_SOURCE")
}
