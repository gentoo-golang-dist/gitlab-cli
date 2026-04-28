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
		name       string
		args       string
		wantErr    string
		wantStdout string
		chdir      bool // chdir to a non-git temp dir
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
			assert.Contains(t, stdout.String(), tt.wantStdout)
		})
	}
}

func TestInstallSkills(t *testing.T) {
	targetDir := t.TempDir()

	installed, err := installSkills(targetDir)
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

func TestInstallSkillsOverwrite(t *testing.T) {
	targetDir := t.TempDir()

	// Install once
	_, err := installSkills(targetDir)
	require.NoError(t, err)

	// Install again — should succeed (silent overwrite)
	installed, err := installSkills(targetDir)
	require.NoError(t, err)
	require.NotEmpty(t, installed)

	// File should still be valid
	skillPath := filepath.Join(targetDir, "glab", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: glab")
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
