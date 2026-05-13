//go:build !integration

package infer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	git_testing "gitlab.com/gitlab-org/cli/internal/git/testing"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestParseCommitSelection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		wantErr  bool
	}{
		{
			name:     "single commit",
			input:    "abc123\n",
			expected: []string{"abc123"},
			wantErr:  false,
		},
		{
			name:     "multiple commits",
			input:    "abc123\ndef456\nghi789\n",
			expected: []string{"abc123", "def456", "ghi789"},
			wantErr:  false,
		},
		{
			name:     "commits with comments",
			input:    "abc123 # Fix critical bug\ndef456 # Add new feature\n",
			expected: []string{"abc123", "def456"},
			wantErr:  false,
		},
		{
			name:     "ignore comment lines",
			input:    "# This is a comment\nabc123\n# Another comment\ndef456\n",
			expected: []string{"abc123", "def456"},
			wantErr:  false,
		},
		{
			name:     "ignore empty lines",
			input:    "abc123\n\ndef456\n\n\nghi789\n",
			expected: []string{"abc123", "def456", "ghi789"},
			wantErr:  false,
		},
		{
			name:     "mixed comments and empty lines",
			input:    "# Choose commits for your stack\nabc123 # First commit\n\n# Second commit below\ndef456\n# End of selection\n",
			expected: []string{"abc123", "def456"},
			wantErr:  false,
		},
		{
			name:     "commits with whitespace",
			input:    "  abc123  \n  def456  # with comment  \n",
			expected: []string{"abc123", "def456"},
			wantErr:  false,
		},
		{
			name:     "invalid format - extra content without comment",
			input:    "abc123 extra content without hash\n",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid format - multiple words without comment",
			input:    "abc123 def456 ghi789\n",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "only comments",
			input:    "# Comment 1\n# Comment 2\n# Comment 3\n",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "only empty lines",
			input:    "\n\n\n",
			expected: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseCommitSelection(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "improperly formatted reorder file")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestHasComment(t *testing.T) {
	tests := []struct {
		name     string
		words    []string
		expected bool
	}{
		{
			name:     "has comment",
			words:    []string{"abc123", "# This is a comment"},
			expected: true,
		},
		{
			name:     "has comment with multiple words",
			words:    []string{"abc123", "# Fix", "the", "bug"},
			expected: true,
		},
		{
			name:     "no comment",
			words:    []string{"abc123", "def456"},
			expected: false,
		},
		{
			name:     "single word",
			words:    []string{"abc123"},
			expected: false,
		},
		{
			name:     "empty slice",
			words:    []string{},
			expected: false,
		},
		{
			name:     "comment not at second position",
			words:    []string{"abc123", "def456", "# comment"},
			expected: false,
		},
		{
			name:     "hash without space",
			words:    []string{"abc123", "#comment"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasComment(tt.words)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptForCommits(t *testing.T) {
	t.Run("returns selected commits from editor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		mockGR.EXPECT().
			Git("log", "--format=%s%x00%h%x00%an", "--reverse", "main..HEAD").
			Return("First commit\x00abc123\x00Author One\nSecond commit\x00def456\x00Author Two", nil)

		var prompts []string
		getText := func(_ context.Context, _, _, content string) (string, error) {
			prompts = append(prompts, content)
			return "abc123\ndef456\n", nil
		}

		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		factory := cmdtest.NewTestFactory(ios)

		commits, err := promptForCommits(t.Context(), factory, getText, mockGR, []string{"main..HEAD"})
		require.NoError(t, err)
		assert.Equal(t, []string{"abc123", "def456"}, commits)
		require.Len(t, prompts, 1)
		assert.Contains(t, prompts[0], "abc123")
		assert.Contains(t, prompts[0], "def456")
	})

	t.Run("filters commits via editor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		mockGR.EXPECT().
			Git("log", "--format=%s%x00%h%x00%an", "--reverse", "HEAD~3..HEAD").
			Return("First\x00aaa111\x00Dev\nSecond\x00bbb222\x00Dev\nThird\x00ccc333\x00Dev", nil)

		getText := func(_ context.Context, _, _, _ string) (string, error) {
			return "aaa111\n# bbb222\nccc333\n", nil
		}

		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(true))
		factory := cmdtest.NewTestFactory(ios)

		commits, err := promptForCommits(t.Context(), factory, getText, mockGR, []string{"HEAD~3..HEAD"})
		require.NoError(t, err)
		assert.Equal(t, []string{"aaa111", "ccc333"}, commits)
	})

	t.Run("returns error with no args", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockGR := git_testing.NewMockGitRunner(ctrl)

		getText := func(_ context.Context, _, _, _ string) (string, error) {
			return "", nil
		}

		ios, _, _, _ := cmdtest.TestIOStreams()
		factory := cmdtest.NewTestFactory(ios)

		_, err := promptForCommits(t.Context(), factory, getText, mockGR, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no revision arguments provided")
	})
}

func TestParseCommitSelectionErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "error contains line content",
			input:       "abc123 invalid content here\n",
			expectedErr: "abc123 invalid content here",
		},
		{
			name:        "error with multiple invalid lines - first one reported",
			input:       "abc123 invalid\ndef456 also invalid\n",
			expectedErr: "abc123 invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCommitSelection(tt.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
