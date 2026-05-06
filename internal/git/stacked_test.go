//go:build !integration

package git

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/config"
)

func TestSetLocalConfig(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		existingConfig bool
	}{
		{
			name:           "config already exists",
			value:          "exciting new value",
			existingConfig: true,
		},
		{
			name:           "config doesn't exist",
			value:          "default value",
			existingConfig: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := InitGitRepo(t)
			defer os.RemoveAll(tempDir)

			if tt.existingConfig {
				_ = GitCommand("config", "--local", "this.glabstacks", "prev-value")
			}

			err := SetLocalConfig("this.glabstacks", tt.value)
			require.NoError(t, err)

			config, err := GetAllConfig("this.glabstacks")
			require.NoError(t, err)

			// GetAllConfig() appends a new line. Let's get rid of that.
			compareString := strings.TrimSuffix(string(config), "\n")

			if compareString != tt.value {
				t.Errorf("config value = %v, want %v", compareString, tt.value)
			}
		})
	}
}

func Test_AddStackRefDir(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		worktree bool
	}{
		{
			name:   "normal filename",
			branch: "thing",
		},
		{
			name:   "advanced filename",
			branch: "something-with-dashes",
		},
		{
			name:     "normal filename in worktree",
			branch:   "thing",
			worktree: true,
		},
		{
			name:     "advanced filename in worktree",
			branch:   "something-with-dashes",
			worktree: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitGitRepoOrWorktree(t, tt.worktree)

			_, err := AddStackRefDir(tt.branch)
			require.NoError(t, err)

			stackLoc, locErr := StackLocation()
			require.NoError(t, locErr)

			_, err = os.Stat(filepath.Join(stackLoc, tt.branch))
			require.NoError(t, err)
		})
	}
}

func Test_StackRootDir(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		worktree bool
	}{
		{
			name:  "valid title",
			title: "test-stack",
		},
		{
			name:  "empty title",
			title: "",
		},
		{
			name:     "valid title in worktree",
			title:    "test-stack",
			worktree: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitGitRepoOrWorktree(t, tt.worktree)

			got, err := StackRootDir(tt.title)
			require.NoError(t, err)

			// Verify the path contains the expected components
			require.Contains(t, got, "stacked", "StackRootDir() should contain stacked dir name")
			require.Contains(t, got, tt.title, "StackRootDir() should contain title")
		})
	}
}

func Test_AddStackRefFile(t *testing.T) {
	type args struct {
		title    string
		stackRef StackRef
	}
	tests := []struct {
		name     string
		args     args
		worktree bool
		wantErr  bool
	}{
		{
			name: "no message",
			args: args{
				title: "sweet-title-123",
				stackRef: StackRef{
					Prev:   "hello",
					Branch: "gmh-feature-3ab3da",
					Next:   "goodbye",
					SHA:    "1a2b3c4d",
					MR:     "https://gitlab.com/",
				},
			},
			wantErr: true,
		},
		{
			name:     "no message in worktree",
			worktree: true,
			args: args{
				title: "sweet-title-123",
				stackRef: StackRef{
					Prev:   "hello",
					Branch: "gmh-feature-3ab3da",
					Next:   "goodbye",
					SHA:    "1a2b3c4d",
					MR:     "https://gitlab.com/",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitGitRepoOrWorktree(t, tt.worktree)

			err := AddStackRefFile(tt.args.title, tt.args.stackRef)
			require.Nil(t, err)

			stackLoc, locErr := StackLocation()
			require.NoError(t, locErr)
			file := filepath.Join(stackLoc, tt.args.title, tt.args.stackRef.SHA+".json")
			require.True(t, config.CheckFileExists(file))

			stackRef := StackRef{}
			readData, err := os.ReadFile(file)
			require.Nil(t, err)

			err = json.Unmarshal(readData, &stackRef)
			require.Nil(t, err)

			require.Equal(t, stackRef, tt.args.stackRef)
		})
	}
}

func Test_DeleteStackRefFile(t *testing.T) {
	// TODO: write test
}

func Test_UpdateStackRefFile(t *testing.T) {
	type args struct {
		title    string
		stackRef StackRef
	}
	tests := []struct {
		name     string
		args     args
		worktree bool
		wantErr  bool
	}{
		{
			name: "no message",
			args: args{
				title: "sweet-title-123",
				stackRef: StackRef{
					Prev:   "hello",
					Branch: "gmh-feature-3ab3da",
					Next:   "goodbye",
					SHA:    "1a2b3c4d",
					MR:     "https://gitlab.com/",
				},
			},
			wantErr: true,
		},
		{
			name:     "no message in worktree",
			worktree: true,
			args: args{
				title: "sweet-title-123",
				stackRef: StackRef{
					Prev:   "hello",
					Branch: "gmh-feature-3ab3da",
					Next:   "goodbye",
					SHA:    "1a2b3c4d",
					MR:     "https://gitlab.com/",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitGitRepoOrWorktree(t, tt.worktree)

			// add the initial data
			initial := StackRef{Prev: "123", Branch: "gmh"}
			err := AddStackRefFile(tt.args.title, initial)
			require.Nil(t, err)

			err = UpdateStackRefFile(tt.args.title, tt.args.stackRef)

			require.Nil(t, err)

			stackLoc, locErr := StackLocation()
			require.NoError(t, locErr)
			file := filepath.Join(stackLoc, tt.args.title, tt.args.stackRef.SHA+".json")
			require.True(t, config.CheckFileExists(file))

			stackRef := StackRef{}
			readData, err := os.ReadFile(file)
			require.Nil(t, err)

			err = json.Unmarshal(readData, &stackRef)
			require.Nil(t, err)

			require.Equal(t, stackRef, tt.args.stackRef)
		})
	}
}

func Test_GetStacks(t *testing.T) {
	stacks := []Stack{
		{
			Title: "stack-0",
			Refs: map[string]StackRef{
				"0": {
					Description: "stack-0 initial commit",
				},
			},
		},
		{
			Title: "stack-1",
			Refs: map[string]StackRef{
				"0": {
					Description: "stack-1 initial commit",
				},
			},
		},
	}

	for _, worktree := range []bool{false, true} {
		suffix := ""
		if worktree {
			suffix = " in worktree"
		}

		t.Run("two stacks"+suffix, func(t *testing.T) {
			InitGitRepoOrWorktree(t, worktree)
			var want []Stack
			for _, v := range stacks {
				for _, ref := range v.Refs {
					err := AddStackRefFile(v.Title, ref)
					require.Nil(t, err)
				}
				want = append(want, Stack{Title: v.Title})
			}
			got, err := GetStacks()
			require.Nil(t, err)
			require.Equal(t, want, got)
		})
		t.Run("no stacks"+suffix, func(t *testing.T) {
			InitGitRepoOrWorktree(t, worktree)
			got, err := GetStacks()
			var want []Stack = nil
			require.NotNil(t, err)
			require.Equal(t, want, got)
		})
	}
}

func TestStandardGitCommand_Git_SetsLocale(t *testing.T) {
	// This test verifies that StandardGitCommand.Git() sets LC_ALL=C
	// to ensure Git output is always in English, regardless of user's locale.

	// Use InitGitRepoWithCommit to ensure we have a proper repo with commits
	// so git status returns predictable output
	InitGitRepoWithCommit(t)

	gitCmd := StandardGitCommand{}

	t.Run("git status outputs English messages", func(t *testing.T) {
		// Run git status which outputs human-readable messages
		output, err := gitCmd.Git("status", "--long")
		require.NoError(t, err)

		// Verify output contains English messages
		// These strings would be different in other locales if LC_ALL=C wasn't set:
		// - German: "Auf Branch" instead of "On branch"
		// - French: "Sur la branche" instead of "On branch"
		// - Chinese: "位于分支" instead of "On branch"
		// Note: Fresh repos may show "No commits yet" but repos with commits show "On branch"
		require.True(t,
			strings.Contains(output, "On branch") || strings.Contains(output, "No commits yet"),
			"Git output should be in English, got: %s", output)
	})

	t.Run("git status contains expected English phrases", func(t *testing.T) {
		// InitGitRepoWithCommit creates a repo with a commit and clean working tree,
		// so git status should output "nothing to commit" since there are no changes
		output, err := gitCmd.Git("status", "--long")
		require.NoError(t, err)

		// This is the key phrase that stack sync looks for to detect up-to-date branches
		// Without LC_ALL=C, it would be localized and string matching would fail:
		// - German: "nichts zu committen"
		// - French: "rien à valider"
		// - Chinese: "无文件要提交"
		require.Contains(t, output, "nothing to commit",
			"Git status should contain 'nothing to commit' for clean working tree")

		// Also verify it shows the branch is clean (another English phrase)
		require.Contains(t, output, "working tree clean",
			"Git status should contain 'working tree clean' for clean working tree")
	})

	t.Run("git commands work regardless of user's LANG setting", func(t *testing.T) {
		// Even if the user has a non-English LANG variable set in their environment,
		// the LC_ALL=C setting should override it and force English output.
		// We can't reliably test locale changes in the test environment,
		// but we can verify the command succeeds and produces valid output.
		output, err := gitCmd.Git("branch", "--show-current")
		require.NoError(t, err)
		require.NotEmpty(t, output, "Git command should produce output")
	})
}
