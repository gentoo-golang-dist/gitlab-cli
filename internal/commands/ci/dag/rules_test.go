//go:build !integration

package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVariables_Branch(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Branch: "main",
	})

	assert.Equal(t, "push", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_COMMIT_BRANCH"])
	assert.Equal(t, "main", vars["CI_DEFAULT_BRANCH"])
	assert.Empty(t, vars["CI_COMMIT_TAG"])
}

func TestBuildVariables_Tag(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Tag: "v2.0.0",
	})

	assert.Equal(t, "push", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "v2.0.0", vars["CI_COMMIT_TAG"])
	assert.Empty(t, vars["CI_COMMIT_BRANCH"])
}

func TestBuildVariables_MergeRequest(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		SourceBranch: "feat/x",
		TargetBranch: "main",
	})

	assert.Equal(t, "merge_request_event", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "feat/x", vars["CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"])
	assert.Equal(t, "main", vars["CI_MERGE_REQUEST_TARGET_BRANCH_NAME"])
	assert.Equal(t, "1", vars["CI_MERGE_REQUEST_IID"])
}

func TestBuildVariables_Schedule(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Source: "schedule",
		Branch: "main",
	})

	assert.Equal(t, "schedule", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_COMMIT_BRANCH"])
}

func TestBuildVariables_WebSource(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Source: "web",
		Branch: "main",
	})

	assert.Equal(t, "web", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_COMMIT_BRANCH"])
}

func TestBuildVariables_ExtraVarsOverride(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Branch: "main",
		ExtraVars: map[string]string{
			"CI_PROJECT_VISIBILITY": "public",
			"CUSTOM_VAR":            "hello",
		},
	})

	assert.Equal(t, "public", vars["CI_PROJECT_VISIBILITY"])
	assert.Equal(t, "hello", vars["CUSTOM_VAR"])
}

func TestEvalWorkflowRules_Accepts(t *testing.T) {
	t.Parallel()

	rules := []RuleClause{
		{If: `$CI_PIPELINE_SOURCE == "merge_request_event"`, When: "on_success"},
		{If: "$CI_COMMIT_TAG", When: "on_success"},
		{If: "$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH", When: "on_success"},
	}

	tests := []struct {
		name string
		vars map[string]string
	}{
		{"MR pipeline", map[string]string{"CI_PIPELINE_SOURCE": "merge_request_event", "CI_DEFAULT_BRANCH": "main"}},
		{"tag pipeline", map[string]string{"CI_COMMIT_TAG": "v1.0", "CI_DEFAULT_BRANCH": "main"}},
		{"default branch", map[string]string{"CI_COMMIT_BRANCH": "main", "CI_DEFAULT_BRANCH": "main"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := EvalWorkflowRules(rules, tt.vars)
			assert.NoError(t, err)
		})
	}
}

func TestEvalWorkflowRules_Rejects(t *testing.T) {
	t.Parallel()

	rules := []RuleClause{
		{If: `$CI_PIPELINE_SOURCE == "merge_request_event"`, When: "on_success"},
		{If: "$CI_COMMIT_TAG", When: "on_success"},
		{If: "$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH", When: "on_success"},
	}

	vars := map[string]string{
		"CI_PIPELINE_SOURCE": "schedule",
		"CI_COMMIT_BRANCH":   "develop",
		"CI_DEFAULT_BRANCH":  "main",
	}

	err := EvalWorkflowRules(rules, vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no workflow:rules matched")
}

func TestEvalWorkflowRules_WhenNever(t *testing.T) {
	t.Parallel()

	rules := []RuleClause{
		{If: `$CI_PIPELINE_SOURCE == "schedule"`, When: "never"},
		{When: "on_success"},
	}

	vars := map[string]string{"CI_PIPELINE_SOURCE": "schedule"}
	err := EvalWorkflowRules(rules, vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "when:never")
}

func TestFilterJobs_BranchPipeline(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "rules_basic.yml"))
	require.NoError(t, err)

	vars := BuildVariables(SimulationConfig{Branch: "main"})
	filtered := FilterJobs(p.Jobs, vars)

	names := jobNames(filtered)
	assert.Contains(t, names, "test-job")           // no rules, runs on branch
	assert.Contains(t, names, "default-branch-job") // matches CI_COMMIT_BRANCH == CI_DEFAULT_BRANCH
	assert.Contains(t, names, "never-on-mr")        // when:never only on MR, falls through to on_success
	assert.NotContains(t, names, "mr-only-job")     // MR-only rule doesn't match
	assert.NotContains(t, names, "tag-only-job")    // tag-only rule doesn't match
	assert.NotContains(t, names, "schedule-job")    // schedule-only rule doesn't match
}

func TestFilterJobs_MRPipeline(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "rules_basic.yml"))
	require.NoError(t, err)

	vars := BuildVariables(SimulationConfig{SourceBranch: "feat/x"})
	filtered := FilterJobs(p.Jobs, vars)

	names := jobNames(filtered)
	assert.Contains(t, names, "mr-only-job")           // MR rule matches
	assert.NotContains(t, names, "test-job")           // no rules, excluded in MR pipelines
	assert.NotContains(t, names, "never-on-mr")        // when:never on MR
	assert.NotContains(t, names, "default-branch-job") // branch rule doesn't match
	assert.NotContains(t, names, "tag-only-job")       // tag rule doesn't match
}

func TestFilterJobs_TagPipeline(t *testing.T) {
	t.Parallel()

	p, err := ParsePipeline(readTestdata(t, "rules_basic.yml"))
	require.NoError(t, err)

	vars := BuildVariables(SimulationConfig{Tag: "v1.0.0"})
	filtered := FilterJobs(p.Jobs, vars)

	names := jobNames(filtered)
	assert.Contains(t, names, "test-job")       // no rules, runs on tag
	assert.Contains(t, names, "tag-only-job")   // tag rule matches
	assert.NotContains(t, names, "mr-only-job") // MR rule doesn't match
}

func TestFilterJobs_NoRulesDefaultBehavior(t *testing.T) {
	t.Parallel()

	jobs := []Job{
		{Name: "no-rules-job", Stage: "test"},
	}

	branchVars := BuildVariables(SimulationConfig{Branch: "main"})
	assert.Len(t, FilterJobs(jobs, branchVars), 1)

	mrVars := BuildVariables(SimulationConfig{SourceBranch: "feat"})
	assert.Empty(t, FilterJobs(jobs, mrVars))
}

func jobNames(jobs []Job) []string {
	var names []string
	for _, j := range jobs {
		names = append(names, j.Name)
	}
	return names
}
