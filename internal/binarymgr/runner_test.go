//go:build !integration

package binarymgr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// runnerFor builds a Runner backed by a blank in-memory config for tests
// that exercise persistence helpers.
func runnerFor(t *testing.T) (*Runner, config.Config) {
	t.Helper()
	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	cfg := config.NewBlankConfig()
	spec := testSpec()
	return &Runner{
		IO:      ios,
		Cfg:     cfg,
		Spec:    spec,
		Manager: NewManager(ios, spec),
	}, cfg
}

func TestRunner_saveAutoDownloadPreference(t *testing.T) {
	t.Parallel()

	t.Run("empty preference is a no-op", func(t *testing.T) {
		t.Parallel()
		r, cfg := runnerFor(t)
		r.saveAutoDownloadPreference("")

		got, _ := cfg.Get("", r.Spec.configKey("auto_download"))
		assert.Empty(t, got, "no preference should be persisted for empty input")
	})

	t.Run("opt-in is persisted", func(t *testing.T) {
		t.Parallel()
		r, cfg := runnerFor(t)
		r.saveAutoDownloadPreference("true")

		got, _ := cfg.Get("", r.Spec.configKey("auto_download"))
		assert.Equal(t, "true", got)
	})
}

func TestRunner_saveLastUpdateCheck(t *testing.T) {
	t.Parallel()

	r, cfg := runnerFor(t)
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	r.saveLastUpdateCheck(now)

	got, _ := cfg.Get("", r.Spec.configKey("last_update_check"))
	require.NotEmpty(t, got)

	parsed, err := time.Parse(time.RFC3339, got)
	require.NoError(t, err)
	assert.True(t, parsed.Equal(now), "expected %s, got %s", now, parsed)
}
