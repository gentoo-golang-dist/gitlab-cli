//go:build !integration

package visualize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildVariables_Branch(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{Branch: "main"})

	assert.Equal(t, "push", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_COMMIT_BRANCH"])
	assert.Equal(t, "main", vars["CI_COMMIT_REF_NAME"])
	assert.Empty(t, vars["CI_COMMIT_TAG"])
}

func TestBuildVariables_Tag(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{Tag: "v2.0.0"})

	assert.Equal(t, "push", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "v2.0.0", vars["CI_COMMIT_TAG"])
	assert.Equal(t, "v2.0.0", vars["CI_COMMIT_REF_NAME"])
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
	assert.Equal(t, "feat/x", vars["CI_COMMIT_REF_NAME"])
	assert.Equal(t, "1", vars["CI_MERGE_REQUEST_IID"])
}

func TestBuildVariables_ExplicitSourceWins(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Source: "schedule",
		Branch: "main",
	})

	assert.Equal(t, "schedule", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "main", vars["CI_COMMIT_BRANCH"])
}

func TestBuildVariables_ExtraVarsOverride(t *testing.T) {
	t.Parallel()

	vars := BuildVariables(SimulationConfig{
		Branch: "main",
		ExtraVars: map[string]string{
			"CI_PIPELINE_SOURCE": "web",
			"MY_CUSTOM":          "value",
		},
	})

	assert.Equal(t, "web", vars["CI_PIPELINE_SOURCE"])
	assert.Equal(t, "value", vars["MY_CUSTOM"])
}
