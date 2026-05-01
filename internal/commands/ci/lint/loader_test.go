//go:build !integration

package lint

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContent(t *testing.T) {
	t.Parallel()

	t.Run("loads local file content", func(t *testing.T) {
		t.Parallel()

		want, err := os.ReadFile(testdataPath(".gitlab-ci.yaml"))
		require.NoError(t, err)

		got, err := loadContent(testdataPath(".gitlab-ci.yaml"))
		require.NoError(t, err)

		assert.Equal(t, want, got)
	})

	t.Run("loads url content", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("variables:\n  REMOTE: true\n"))
		}))
		defer server.Close()

		got, err := loadContent(server.URL)
		require.NoError(t, err)

		assert.Equal(t, "variables:\n  REMOTE: true\n", string(got))
	})

	t.Run("returns a missing-file error", func(t *testing.T) {
		t.Parallel()

		_, err := loadContent("WRONG_PATH")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "WRONG_PATH: no such file or directory")
	})

	t.Run("returns url fetch errors", func(t *testing.T) {
		t.Parallel()

		_, err := loadURLContent("://bad-url")
		require.Error(t, err)
	})
}
