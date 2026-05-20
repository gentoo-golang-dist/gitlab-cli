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

type lintStub struct {
	t              *testing.T
	mu             sync.Mutex
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
			var req struct {
				Content string `json:"content"`
			}
			_ = json.Unmarshal(body, &req)

			s.mu.Lock()
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
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		default:
			s.t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

func (s *lintStub) setForceLintError(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forceLintError = v
}

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
