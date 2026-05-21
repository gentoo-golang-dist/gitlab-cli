package mrutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultHost = "gitlab.com"

func TestParseMRRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantIID  int
		wantRepo string // expected Repo.FullName(), "" if Repo should be nil
		wantHost string // expected Repo.RepoHost(), "" if Repo should be nil
		wantErr  string
	}{
		{name: "bare iid", input: "42", wantIID: 42},
		{name: "whitespace trimmed", input: "  3  ", wantIID: 3},
		{
			name:     "default host URL",
			input:    "https://gitlab.com/group/project/-/merge_requests/7",
			wantIID:  7,
			wantRepo: "group/project",
			wantHost: "gitlab.com",
		},
		{
			name:     "self-hosted URL",
			input:    "https://gitlab.example.com/group/sub/project/-/merge_requests/12",
			wantIID:  12,
			wantRepo: "group/sub/project",
			wantHost: "gitlab.example.com",
		},

		{name: "empty", input: "", wantErr: "empty merge request reference"},
		{name: "zero iid", input: "0", wantErr: "invalid merge request reference"},
		{name: "negative iid", input: "-1", wantErr: "invalid merge request reference"},
		{name: "bare non-numeric", input: "foo", wantErr: "invalid merge request reference"},
		{name: "bang sigil rejected", input: "!42", wantErr: "invalid merge request reference"},
		{name: "project bang rejected", input: "group/project!42", wantErr: "invalid merge request reference"},
		{name: "non-MR URL rejected", input: "https://gitlab.com/group/project/-/issues/5", wantErr: "invalid merge request reference"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMRRef(tc.input, defaultHost)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantIID, got.IID)
			if tc.wantRepo == "" {
				assert.Nil(t, got.Repo)
			} else {
				require.NotNil(t, got.Repo)
				assert.Equal(t, tc.wantRepo, got.Repo.FullName())
				assert.Equal(t, tc.wantHost, got.Repo.RepoHost())
			}
		})
	}
}

func TestParseMRRefs(t *testing.T) {
	t.Run("all valid", func(t *testing.T) {
		got, err := ParseMRRefs([]string{"1", "2", "https://gitlab.com/group/project/-/merge_requests/3"}, defaultHost)
		require.NoError(t, err)
		require.Len(t, got, 3)
		assert.Equal(t, 1, got[0].IID)
		assert.Nil(t, got[0].Repo)
		assert.Equal(t, 2, got[1].IID)
		assert.Nil(t, got[1].Repo)
		assert.Equal(t, 3, got[2].IID)
		require.NotNil(t, got[2].Repo)
		assert.Equal(t, "group/project", got[2].Repo.FullName())
	})

	t.Run("fails fast on first bad ref", func(t *testing.T) {
		_, err := ParseMRRefs([]string{"1", "bogus", "3"}, defaultHost)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bogus")
	})

	t.Run("empty input", func(t *testing.T) {
		got, err := ParseMRRefs(nil, defaultHost)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
