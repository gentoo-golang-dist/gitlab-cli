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
