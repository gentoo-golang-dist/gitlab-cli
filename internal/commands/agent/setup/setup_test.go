//go:build !integration

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdSetup(t *testing.T) {
	tests := []struct {
		name         string
		args         string
		wantErr      string
		wantStdout   string
		wantStderr   string
		noWarnStderr bool // assert stderr is empty
		preInstall   bool // pre-install skills before running
		chdir        bool // chdir to a non-git temp dir
	}{
		{
			name:       "with --path flag",
			args:       "setup --path %s",
			wantStdout: "Installed",
		},
		{
			name:       "with --global flag",
			args:       "setup --global",
			wantStdout: "Installed",
		},
		{
			name:    "default scope outside git repo",
			args:    "setup",
			wantErr: "not in a Git repository",
			chdir:   true,
		},
		{
			name:    "global and path are mutually exclusive",
			args:    "setup --global --path /tmp/skills",
			wantErr: "if any flags in the group [global path] are set none of the others can be",
		},
		{
			name:       "with --force overwrites existing skills",
			args:       "setup --force --path %s",
			preInstall: true,
			wantStdout: "Installed",
		},
		{
			name:       "without --force skips existing skills",
			args:       "setup --path %s",
			preInstall: true,
			wantStderr: "already exists. Use --force to overwrite",
		},
		{
			name:         "without --force fresh install shows no already-exists warnings",
			args:         "setup --path %s",
			wantStdout:   "Installed",
			noWarnStderr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, stdout, stderr := cmdtest.TestIOStreams()

			// chdir to a non-git directory for tests that need it
			if tt.chdir {
				t.Chdir(t.TempDir())
			}

			// For --path tests, substitute a temp dir
			args := tt.args
			if strings.Contains(args, "%s") {
				tmpDir := t.TempDir()
				args = strings.Replace(args, "%s", tmpDir, 1)

				// Pre-install for tests that need existing files
				if tt.preInstall {
					_, err := installSkills(tmpDir, false)
					require.NoError(t, err)
				}
			}

			// For --global tests, override HOME to a temp dir
			if strings.Contains(args, "--global") {
				tmpHome := t.TempDir()
				t.Setenv("HOME", tmpHome)
			}

			f := cmdtest.NewTestFactory(ios)
			setupCmd := NewCmdSetup(f)

			rootCmd := &cobra.Command{Use: "glab"}
			agentCmd := &cobra.Command{Use: "agent"}
			rootCmd.AddCommand(agentCmd)
			agentCmd.AddCommand(setupCmd)

			argv, err := shlex.Split(args)
			require.NoError(t, err)
			rootCmd.SetArgs(append([]string{"agent"}, argv...))

			err = rootCmd.ExecuteContext(t.Context())

			if tt.wantErr != "" {
				require.Error(t, err)
				errMsg := err.Error()
				if stderr != nil {
					errMsg += stderr.String()
				}
				assert.Contains(t, errMsg, tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantStdout != "" {
				assert.Contains(t, stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" {
				assert.Contains(t, stderr.String(), tt.wantStderr)
			}
			if tt.noWarnStderr {
				assert.Empty(t, stderr.String(), "expected no stderr output on fresh install")
			}
		})
	}
}

func TestInstallSkills(t *testing.T) {
	targetDir := t.TempDir()

	installed, err := installSkills(targetDir, false)
	require.NoError(t, err)

	// Should install at least one file
	require.NotEmpty(t, installed)

	// Verify glab/SKILL.md was created
	skillPath := filepath.Join(targetDir, "glab", "SKILL.md")
	assert.FileExists(t, skillPath)

	// Verify content has valid frontmatter
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "---")
	assert.Contains(t, string(content), "name: glab")
	assert.Contains(t, string(content), "description:")
}

func TestInstallSkillsOverwriteWithForce(t *testing.T) {
	targetDir := t.TempDir()

	// Install once
	_, err := installSkills(targetDir, false)
	require.NoError(t, err)

	// Install again with force — should succeed and overwrite
	installed, err := installSkills(targetDir, true)
	require.NoError(t, err)
	require.NotEmpty(t, installed)

	// File should still be valid
	skillPath := filepath.Join(targetDir, "glab", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: glab")
}

func TestInstallSkillsSkipsWithoutForce(t *testing.T) {
	targetDir := t.TempDir()

	// Install once
	_, err := installSkills(targetDir, false)
	require.NoError(t, err)

	// Install again without force — should return no installed paths
	installed, err := installSkills(targetDir, false)
	require.NoError(t, err)
	require.Empty(t, installed)
}

func TestResolveTargetDir(t *testing.T) {
	t.Run("with --path", func(t *testing.T) {
		opts := &options{path: "/custom/path"}
		dir, err := resolveTargetDir(opts)
		require.NoError(t, err)
		assert.Equal(t, "/custom/path", dir)
	})

	t.Run("with --global", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		opts := &options{global: true}
		dir, err := resolveTargetDir(opts)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tmpHome, ".agents", "skills"), dir)
	})
}
