//go:build !integration

package update

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

// newBannerFactory builds a test factory with the supplied build info, a fresh
// blank config seeded with last_seen_version, and a non-TTY stderr buffer.
func newBannerFactory(t *testing.T, currentVersion, lastSeen, showWhatsNew string, codingAgent string) (cmdutils.Factory, *bytes.Buffer) {
	t.Helper()
	t.Setenv("NO_COLOR", "true")

	cfg := config.NewBlankConfig()
	if lastSeen != "" {
		require.NoError(t, cfg.Set("", LastSeenVersionKey, lastSeen))
	}
	if showWhatsNew != "" {
		require.NoError(t, cfg.Set("", "show_whats_new", showWhatsNew))
	}

	ios, _, _, stderr := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	f := cmdtest.NewTestFactory(ios,
		cmdtest.WithConfig(cfg),
		cmdtest.WithBuildInfo(api.BuildInfo{Version: currentVersion, CodingAgent: codingAgent}),
	)
	return f, stderr
}

func TestMaybeShowPostUpgradeBanner(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		lastSeen       string
		showWhatsNew   string
		codingAgent    string
		envOverride    string
		wantBanner     bool
		wantLastSeen   string
	}{
		{
			name:           "no stored value, current newer than seeded default fires banner",
			currentVersion: "1.101.0",
			lastSeen:       "",
			wantBanner:     true,
			wantLastSeen:   "1.101.0",
		},
		{
			name:           "no stored value, current at seeded default is silent",
			currentVersion: "1.100.0",
			lastSeen:       "",
			wantBanner:     false,
			// defaultFor returns "v1.100.0" when nothing is stored — exposed via Get
			wantLastSeen: "v1.100.0",
		},
		{
			name:           "stored value older than current fires banner and updates marker",
			currentVersion: "1.85.0",
			lastSeen:       "1.84.0",
			wantBanner:     true,
			wantLastSeen:   "1.85.0",
		},
		{
			name:           "same version is silent",
			currentVersion: "1.85.0",
			lastSeen:       "1.85.0",
			wantBanner:     false,
			wantLastSeen:   "1.85.0",
		},
		{
			name:           "downgrade is silent",
			currentVersion: "1.84.0",
			lastSeen:       "1.85.0",
			wantBanner:     false,
			wantLastSeen:   "1.85.0",
		},
		{
			name:           "config opt-out suppresses banner",
			currentVersion: "1.85.0",
			lastSeen:       "1.84.0",
			showWhatsNew:   "false",
			wantBanner:     false,
			wantLastSeen:   "1.84.0",
		},
		{
			name:           "env override suppresses banner",
			currentVersion: "1.85.0",
			lastSeen:       "1.84.0",
			envOverride:    "false",
			wantBanner:     false,
			wantLastSeen:   "1.84.0",
		},
		{
			name:           "coding agent is silent",
			currentVersion: "1.85.0",
			lastSeen:       "1.84.0",
			codingAgent:    "claude-code",
			wantBanner:     false,
			wantLastSeen:   "1.84.0",
		},
		{
			name:           "unparseable current version is silent",
			currentVersion: "DEV",
			lastSeen:       "1.84.0",
			wantBanner:     false,
			wantLastSeen:   "1.84.0",
		},
		{
			name:           "unparseable last_seen is silent",
			currentVersion: "1.85.0",
			lastSeen:       "garbage",
			wantBanner:     false,
			wantLastSeen:   "garbage",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer config.StubWriteConfig(io.Discard, io.Discard)()
			if tc.envOverride != "" {
				t.Setenv("GLAB_SHOW_WHATS_NEW", tc.envOverride)
			}

			f, stderr := newBannerFactory(t, tc.currentVersion, tc.lastSeen, tc.showWhatsNew, tc.codingAgent)
			MaybeShowPostUpgradeBanner(f)

			if tc.wantBanner {
				assert.Contains(t, stderr.String(), "What's new in glab")
				assert.Contains(t, stderr.String(), tc.currentVersion)
				assert.Contains(t, stderr.String(), "glab whatsnew")
			} else {
				assert.Empty(t, stderr.String())
			}

			got, _ := f.Config().Get("", LastSeenVersionKey)
			assert.Equal(t, tc.wantLastSeen, got)
		})
	}
}
