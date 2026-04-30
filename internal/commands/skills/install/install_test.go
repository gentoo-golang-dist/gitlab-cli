//go:build !integration

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdInstall(t *testing.T) {
	tests := []struct {
		name         string
		args         string
		wantErr      string
		wantStdout   string
		wantStderr   string
		noWarnStderr bool // assert stderr is empty
		preInstall   bool // pre-install skills before running
		chdir        bool // chdir to a non-git temp dir
		global       bool // test uses --global (override HOME)
		noParallel   bool // cannot parallelize tests using t.Chdir or t.Setenv
	}{
		{
			name:       "with --path flag",
			args:       "--path %s",
			wantStdout: "Installed",
		},
		{
			name:       "with --global flag",
			args:       "--global",
			global:     true,
			noParallel: true,
			wantStdout: "Installed",
		},
		{
			name:       "default scope outside git repo",
			args:       "",
			wantErr:    "not in a Git repository",
			chdir:      true,
			noParallel: true,
		},
		{
			name:    "global and path are mutually exclusive",
			args:    "--global --path /tmp/skills",
			wantErr: "if any flags in the group [global path] are set none of the others can be",
		},
		{
			name:       "with --force overwrites existing skills",
			args:       "--force --path %s",
			preInstall: true,
			wantStdout: "Installed",
		},
		{
			name:       "without --force skips existing skills",
			args:       "--path %s",
			preInstall: true,
			wantStderr: "already exists. Use --force to overwrite",
		},
		{
			name:         "without --force fresh install shows no already-exists warnings",
			args:         "--path %s",
			wantStdout:   "Installed",
			noWarnStderr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.noParallel {
				t.Parallel()
			}

			// chdir to a non-git directory for tests that need it
			if tt.chdir {
				t.Chdir(t.TempDir())
			}

			// For --path tests, substitute a temp dir
			args := tt.args
			if tt.preInstall || containsPathPlaceholder(args) {
				tmpDir := t.TempDir()
				args = fmt.Sprintf(args, tmpDir)

				// Pre-install for tests that need existing files
				if tt.preInstall {
					_, err := installSkills(tmpDir, false)
					require.NoError(t, err)
				}
			}

			// For --global tests, override HOME to a temp dir
			if tt.global {
				t.Setenv("HOME", t.TempDir())
			}

			exec := cmdtest.SetupCmdForTest(t, NewCmdInstall, false)
			out, err := exec(args)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			if tt.wantStdout != "" {
				assert.Contains(t, out.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" {
				assert.Contains(t, out.Stderr(), tt.wantStderr)
			}
			if tt.noWarnStderr {
				assert.Empty(t, out.Stderr(), "expected no stderr output on fresh install")
			}
		})
	}
}

func TestInstallSkills(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	t.Run("with --path", func(t *testing.T) {
		t.Parallel()

		opts := &options{path: "/custom/path"}
		dir, err := resolveTargetDir(opts)
		require.NoError(t, err)
		assert.Equal(t, "/custom/path", dir)
	})

	t.Run("with --global", func(t *testing.T) {
		t.Parallel()

		// resolveTargetDir uses os.UserHomeDir which reads HOME.
		// We verify the result ends with the expected suffix rather
		// than setting HOME (which prevents t.Parallel).
		opts := &options{global: true}
		dir, err := resolveTargetDir(opts)
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(dir), "expected absolute path, got %s", dir)
		assert.Equal(t, filepath.Join(".agents", "skills"), dir[len(dir)-len(filepath.Join(".agents", "skills")):])
	})
}

func containsPathPlaceholder(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' && s[i+1] == 's' {
			return true
		}
	}
	return false
}
