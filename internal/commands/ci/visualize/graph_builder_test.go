//go:build !integration

package visualize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildD2Source_StageContainers(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"build", "test"},
		Jobs: []Job{
			{Name: "compile", Stage: "build"},
			{Name: "unit-tests", Stage: "test"},
		},
	}

	src := BuildD2Source(p, false)

	assert.Contains(t, src, `"stage:build"`)
	assert.Contains(t, src, `label: "build"`)
	assert.Contains(t, src, `"stage:test"`)
	assert.Contains(t, src, `label: "test"`)
}

func TestBuildD2Source_JobNodes(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"build"},
		Jobs: []Job{
			{Name: "compile", Stage: "build"},
			{Name: "trigger-deploy", Stage: "build", IsTrigger: true},
		},
	}

	src := BuildD2Source(p, false)

	assert.Contains(t, src, `"compile"`)
	assert.Contains(t, src, `class: job`)
	assert.Contains(t, src, `"trigger-deploy"`)
	assert.Contains(t, src, `class: trigger_job`)
}

func TestBuildD2Source_NeedsEdges(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"build", "test"},
		Jobs: []Job{
			{Name: "compile", Stage: "build"},
			{Name: "unit-tests", Stage: "test", Needs: []string{"compile"}},
		},
	}

	src := BuildD2Source(p, false)

	assert.Contains(t, src, `"stage:build"."compile" -> "stage:test"."unit-tests": {class: needs_edge}`)
}

func TestBuildD2Source_StageEdges(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"build", "test", "deploy"},
		Jobs: []Job{
			{Name: "compile", Stage: "build"},
			{Name: "test", Stage: "test"},
			{Name: "deploy", Stage: "deploy"},
		},
	}

	srcWith := BuildD2Source(p, true)
	assert.Contains(t, srcWith, `"stage:build" -> "stage:test": {class: stage_edge}`)
	assert.Contains(t, srcWith, `"stage:test" -> "stage:deploy": {class: stage_edge}`)

	srcWithout := BuildD2Source(p, false)
	assert.NotContains(t, srcWithout, `-> "stage:test": {class: stage_edge}`)
}

func TestBuildD2Source_SkipsMissingNeedTarget(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"test"},
		Jobs: []Job{
			{Name: "unit-tests", Stage: "test", Needs: []string{"nonexistent"}},
		},
	}

	src := BuildD2Source(p, false)

	// Should not contain an edge to a nonexistent job.
	assert.NotContains(t, src, "nonexistent")
}

func TestBuildD2Source_DirectionRight(t *testing.T) {
	t.Parallel()

	p := &Pipeline{
		Stages: []string{"build"},
		Jobs:   []Job{{Name: "compile", Stage: "build"}},
	}

	src := BuildD2Source(p, false)
	assert.Contains(t, src, "direction: right")
}
