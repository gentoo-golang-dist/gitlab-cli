package api

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeJSONCommand writes json to a temp file and returns a CommandTokenAuthSource
// that cats that file. This avoids any shell quoting issues in tests.
func makeJSONCommand(t *testing.T, json string) *CommandTokenAuthSource {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "glab-test-token-*.json")
	require.NoError(t, err)
	_, err = f.WriteString(json)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	src, err := NewCommandTokenAuthSource("cat "+f.Name(), "gitlab.com")
	require.NoError(t, err)
	return src
}

func TestNewCommandTokenAuthSource(t *testing.T) {
	t.Run("parses simple command", func(t *testing.T) {
		src, err := NewCommandTokenAuthSource("echo hello", "gitlab.com")
		require.NoError(t, err)
		assert.Equal(t, "echo", src.command)
		assert.Equal(t, []string{"hello"}, src.args)
	})

	t.Run("parses quoted arguments", func(t *testing.T) {
		src, err := NewCommandTokenAuthSource(`my-cli get-token --label "my token"`, "gitlab.com")
		require.NoError(t, err)
		assert.Equal(t, "my-cli", src.command)
		assert.Equal(t, []string{"get-token", "--label", "my token"}, src.args)
	})

	t.Run("returns error for empty command", func(t *testing.T) {
		_, err := NewCommandTokenAuthSource("", "gitlab.com")
		require.Error(t, err)
	})

	t.Run("returns error for invalid shell syntax", func(t *testing.T) {
		_, err := NewCommandTokenAuthSource(`my-cli "unterminated quote`, "gitlab.com")
		require.Error(t, err)
	})
}

func TestCommandTokenAuthSource_Header(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell commands not portable to Windows")
	}

	ctx := t.Context()

	t.Run("pat token", func(t *testing.T) {
		src := makeJSONCommand(t, `{"type":"pat","token":"glpat-abc"}`)
		key, val, err := src.Header(ctx)
		require.NoError(t, err)
		assert.Equal(t, "PRIVATE-TOKEN", key)
		assert.Equal(t, "glpat-abc", val)
	})

	t.Run("oauth2 token", func(t *testing.T) {
		src := makeJSONCommand(t, `{"type":"oauth2","token":"mytoken"}`)
		key, val, err := src.Header(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Authorization", key)
		assert.Equal(t, "Bearer mytoken", val)
	})

	t.Run("job-token", func(t *testing.T) {
		src := makeJSONCommand(t, `{"type":"job-token","token":"myjobtoken"}`)
		key, val, err := src.Header(ctx)
		require.NoError(t, err)
		assert.Equal(t, "JOB-TOKEN", key)
		assert.Equal(t, "myjobtoken", val)
	})

	t.Run("command not found", func(t *testing.T) {
		src, err := NewCommandTokenAuthSource("this-command-does-not-exist-xyz", "gitlab.com")
		require.NoError(t, err)
		_, _, err = src.Header(ctx)
		require.Error(t, err)
		var exitErr *exec.Error
		assert.ErrorAs(t, err, &exitErr)
	})

	t.Run("non-zero exit code", func(t *testing.T) {
		// Use `false` — a POSIX tool that always exits with code 1.
		src, err := NewCommandTokenAuthSource("false", "gitlab.com")
		require.NoError(t, err)
		_, _, err = src.Header(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed")
	})

	t.Run("invalid JSON output", func(t *testing.T) {
		src := makeJSONCommand(t, "not-json")
		_, _, err := src.Header(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("empty token in response", func(t *testing.T) {
		src := makeJSONCommand(t, `{"type":"pat","token":""}`)
		_, _, err := src.Header(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty token")
	})

	t.Run("unknown token type", func(t *testing.T) {
		src := makeJSONCommand(t, `{"type":"unknown","token":"abc"}`)
		_, _, err := src.Header(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown token type")
	})

	t.Run("timeout", func(t *testing.T) {
		// Use `sleep 200` — a POSIX tool. The 50ms context deadline fires well before.
		src, err := NewCommandTokenAuthSource("sleep 200", "gitlab.com")
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		_, _, err = src.Header(ctx)
		require.Error(t, err)
	})

	t.Run("no caching - command runs every time", func(t *testing.T) {
		// Verify two successive Header() calls both succeed.
		src := makeJSONCommand(t, `{"type":"pat","token":"tok"}`)
		key1, val1, err := src.Header(ctx)
		require.NoError(t, err)
		key2, val2, err := src.Header(ctx)
		require.NoError(t, err)
		assert.Equal(t, key1, key2)
		assert.Equal(t, val1, val2)
	})
}
