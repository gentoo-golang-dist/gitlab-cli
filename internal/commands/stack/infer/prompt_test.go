//go:build !integration

package infer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/commands/stack/stackutils"
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
			result := stackutils.HasComment(tt.words)
			assert.Equal(t, tt.expected, result)
		})
	}
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
