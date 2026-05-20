//go:build !integration

package setup

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/orbit/internal/orbiterr"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// stubSkill returns a multi-file skill that mimics the on-disk layout
// the real orbit registry entry produces (SKILL.md + a references/
// directory). Exists in tests only — the real Get() call hits the
// network and is exercised by the skills/remote package tests.
func stubSkill() skill.Skill {
	return skill.Skill{
		Name:        "orbit",
		Description: "stub",
		Source:      skill.SourceRemote,
		Files: map[string][]byte{
			"SKILL.md":                      []byte("# stub skill"),
			"references/troubleshooting.md": []byte("stub reference"),
		},
	}
}

// healthyStatus is the canonical Orbit response used across happy-path tests.
func healthyStatus() *gitlab.OrbitStatus {
	return &gitlab.OrbitStatus{
		Status:    "healthy",
		Timestamp: "2026-05-20T12:00:00Z",
		Version:   "0.6.0",
	}
}

func okResponse() *gitlab.Response {
	return &gitlab.Response{Response: &http.Response{StatusCode: http.StatusOK}}
}

func TestSetup_Structure(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	cmd := NewCmd(cmdtest.NewTestFactory(ios))

	for _, name := range []string{"yes", "global", "path", "upgrade", "skip-skill", "skip-local", "hostname"} {
		assert.NotNilf(t, cmd.Flags().Lookup(name), "--%s should be registered", name)
	}
	assert.Equal(t, "true", cmd.Annotations["mcp:safe"])
}

// TestSetup_StatusOnly_HappyPath verifies that reachability succeeds and
// the skill / local steps are cleanly skipped via the opt-out flags.
func TestSetup_StatusOnly_HappyPath(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetStatus(gomock.Any(), gomock.Any()).
		Return(healthyStatus(), okResponse(), nil)

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	out, err := exec("--skip-skill --skip-local")
	require.NoError(t, err)
	// LogInfof writes to stdout (StdOut), not stderr.
	stdout := out.OutBuf.String()
	assert.Contains(t, stdout, "Checking Orbit reachability")
	assert.Contains(t, stdout, "Orbit reachable")
	assert.Contains(t, stdout, "0.6.0")
}

// TestSetup_ShortCircuitOnFFOff confirms that a 404 from /orbit/status
// stops the flow with the orbiterr exit code, before any skill or
// binary work happens.
func TestSetup_ShortCircuitOnFFOff(t *testing.T) {
	t.Parallel()

	testClient := gitlabtesting.NewTestClient(t)
	testClient.MockOrbit.EXPECT().
		GetStatus(gomock.Any(), gomock.Any()).
		Return(nil,
			&gitlab.Response{Response: &http.Response{StatusCode: http.StatusNotFound}},
			&gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "404 Not Found",
			})

	exec := cmdtest.SetupCmdForTest(
		t,
		NewCmd,
		false,
		cmdtest.WithApiClient(cmdtest.NewTestApiClient(t, nil, "", "", api.WithGitLabClient(testClient.Client))),
	)

	// Pass --skip-skill/--skip-local so a regression that ran them
	// before status would be caught: the test would still fail with a
	// different error.
	_, err := exec("")
	require.Error(t, err)

	var exitErr *cmdutils.ExitError
	require.True(t, errors.As(err, &exitErr), "error should be *cmdutils.ExitError, got %T", err)
	assert.Equal(t, orbiterr.ExitOrbitUnavailable, exitErr.Code)
}

// TestSetup_SkillScopeResolution covers the three skill-target paths
// directly via complete() rather than the full cobra round-trip. The
// real Get() call for the orbit skill hits gitlab.com, so end-to-end
// install is left to the skills/remote package's network-mocked tests.
//
// Not t.Parallel(): the --global case uses t.Setenv, which forbids
// parallelism for the entire test.
func TestSetup_SkillScopeResolution(t *testing.T) {
	t.Run("--path is used verbatim", func(t *testing.T) {
		o := &options{path: "/explicit/target"}
		require.NoError(t, o.complete())
		assert.Equal(t, "/explicit/target", o.skillTargetDir)
	})

	t.Run("--global resolves to ~/.agents/skills/", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		o := &options{global: true}
		require.NoError(t, o.complete())
		assert.Equal(t, filepath.Join(home, skillsRelDir), o.skillTargetDir)
	})

	t.Run("--skip-skill leaves target unset", func(t *testing.T) {
		o := &options{skipSkill: true}
		require.NoError(t, o.complete())
		assert.Empty(t, o.skillTargetDir)
	})
}

// TestSetup_WriteSkillIdempotent covers the file-write helper without
// touching the network: a pre-populated skill writes once and then
// no-ops on the second call without --upgrade.
func TestSetup_WriteSkillIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	s := stubSkill()

	require.NoError(t, writeSkill(dir, s, false))
	require.FileExists(t, filepath.Join(dir, s.Name, "SKILL.md"))
	require.FileExists(t, filepath.Join(dir, s.Name, "references/troubleshooting.md"))

	mainPath := filepath.Join(dir, s.Name, "SKILL.md")
	require.NoError(t, os.WriteFile(mainPath, []byte("user edit"), 0o644))

	// Second call without force should NOT overwrite.
	require.NoError(t, writeSkill(dir, s, false))
	got, err := os.ReadFile(mainPath)
	require.NoError(t, err)
	assert.Equal(t, "user edit", string(got))

	// With force=true, the helper restores the bundled content.
	require.NoError(t, writeSkill(dir, s, true))
	got, err = os.ReadFile(mainPath)
	require.NoError(t, err)
	assert.Equal(t, string(s.Files["SKILL.md"]), string(got))
}

// TestSetup_GlobalAndPathMutuallyExclusive locks in the cobra wiring.
func TestSetup_GlobalAndPathMutuallyExclusive(t *testing.T) {
	t.Parallel()

	exec := cmdtest.SetupCmdForTest(t, NewCmd, false)
	_, err := exec("--global --path /tmp/skills")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group [global path] are set none of the others can be")
}
