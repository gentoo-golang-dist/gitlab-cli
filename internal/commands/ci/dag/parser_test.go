//go:build !integration

package dag

import (
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(path.Join(path.Dir(filename), "testdata", name))
	require.NoError(t, err)

	return data
}

func TestParsePipeline_Simple(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "simple.yml"))
	require.NoError(t, err)

	assert.Equal(t, []string{"build", "test", "deploy"}, p.Stages)
	assert.Len(t, p.Jobs, 4)

	jobsByName := jobMap(p.Jobs)

	assert.Equal(t, "build", jobsByName["compile"].Stage)
	assert.Equal(t, "test", jobsByName["unit-tests"].Stage)
	assert.Equal(t, "test", jobsByName["lint"].Stage)
	assert.Equal(t, "deploy", jobsByName["deploy-prod"].Stage)
}

func TestParsePipeline_Needs(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "needs.yml"))
	require.NoError(t, err)

	jobsByName := jobMap(p.Jobs)

	// String form: needs: ["compile"]
	assert.Equal(t, []string{"compile"}, jobsByName["unit-tests"].Needs)

	// Object form: needs: [{job: "compile", artifacts: true}]
	assert.Equal(t, []string{"compile"}, jobsByName["integration-tests"].Needs)

	// Multiple needs.
	assert.ElementsMatch(t, []string{"unit-tests", "integration-tests"}, jobsByName["deploy-prod"].Needs)
}

func TestParsePipeline_HiddenJobs(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "hidden_jobs.yml"))
	require.NoError(t, err)

	// Should only have 2 real jobs, not the .base or .test-template.
	assert.Len(t, p.Jobs, 2)

	jobsByName := jobMap(p.Jobs)
	assert.Contains(t, jobsByName, "compile")
	assert.Contains(t, jobsByName, "test")
	assert.NotContains(t, jobsByName, ".base")
	assert.NotContains(t, jobsByName, ".test-template")
}

func TestParsePipeline_DefaultStages(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "default_stages.yml"))
	require.NoError(t, err)

	// Should use the default stages, filtered to those with jobs.
	assert.Equal(t, []string{"build", "test", "deploy"}, p.Stages)
	assert.Len(t, p.Jobs, 3)
}

func TestParsePipeline_Complex(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "complex.yml"))
	require.NoError(t, err)

	assert.Equal(t, []string{"build", "test", "staging", "deploy"}, p.Stages)

	jobsByName := jobMap(p.Jobs)

	// Hidden job .template should be excluded.
	assert.NotContains(t, jobsByName, ".template")

	// Reserved key "variables" should not be treated as a job.
	assert.NotContains(t, jobsByName, "variables")

	// Trigger job detection.
	assert.True(t, jobsByName["trigger-downstream"].IsTrigger)
	assert.False(t, jobsByName["compile"].IsTrigger)

	// Dependencies.
	assert.Equal(t, []string{"compile"}, jobsByName["unit-tests"].Dependencies)
}

func TestParsePipeline_DefaultStageForJob(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline([]byte(`
stages:
  - test

my-job:
  script: echo hello
`))
	require.NoError(t, err)

	assert.Len(t, p.Jobs, 1)
	assert.Equal(t, "test", p.Jobs[0].Stage)
}

func TestParsePipeline_InvalidYAML(t *testing.T) {
	t.Parallel()

	_, err := ParsePipeline([]byte(`not: [valid: yaml`))
	assert.Error(t, err)
}

func TestParsePipeline_EmptyJobs(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline([]byte(`
stages:
  - build
variables:
  FOO: bar
`))
	require.NoError(t, err)
	assert.Empty(t, p.Jobs)
	assert.Empty(t, p.Stages)
}

func TestParsePipeline_Extends(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "extends.yml"))
	require.NoError(t, err)

	assert.Equal(t, []string{"documentation", "test", "build"}, p.Stages)

	jobsByName := jobMap(p.Jobs)

	// Jobs extending .documentation should inherit stage: documentation.
	assert.Equal(t, "documentation", jobsByName["check_docs_update"].Stage)
	assert.Equal(t, "documentation", jobsByName["check_docs_markdown"].Stage)

	// Inherited needs: [] from .documentation.
	assert.Empty(t, jobsByName["check_docs_update"].Needs)

	// Jobs extending multiple templates: .go-cache then .test.
	// .test has stage: test, which should win over .go-cache (no stage).
	assert.Equal(t, "test", jobsByName["lint"].Stage)
	assert.Equal(t, "test", jobsByName["tests:unit"].Stage)

	// Job with extends but also its own stage: should use its own.
	assert.Equal(t, "build", jobsByName["build_windows"].Stage)
}

func TestParsePipeline_ExtendsChain(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline([]byte(`
stages:
  - deploy

.base:
  stage: deploy

.child:
  extends: .base
  script: echo hello

deploy-job:
  extends: .child
  script: echo deploy
`))
	require.NoError(t, err)

	assert.Len(t, p.Jobs, 1)
	assert.Equal(t, "deploy", p.Jobs[0].Stage)
}

func jobMap(jobs []Job) map[string]Job {
	m := make(map[string]Job)
	for _, j := range jobs {
		m[j.Name] = j
	}
	return m
}
