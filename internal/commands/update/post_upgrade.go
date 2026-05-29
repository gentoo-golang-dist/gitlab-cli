package update

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-version"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

const LastSeenVersionKey = "last_seen_version"

// MaybeShowPostUpgradeBanner prints a one-time nudge pointing at
// `glab whatsnew` whenever buildInfo.Version is newer than the recorded
// LastSeenVersionKey marker. The default for LastSeenVersionKey is seeded
// in defaultFor() so existing users see the banner the first time they
// run the release that ships this feature.
func MaybeShowPostUpgradeBanner(io *iostreams.IOStreams, cfg config.Config, buildInfo api.BuildInfo) {
	if !isWhatsNewEnabled(cfg) {
		return
	}
	if buildInfo.CodingAgent != "" {
		return
	}

	currentVersion := strings.TrimSpace(buildInfo.Version)
	current, err := version.NewVersion(currentVersion)
	if err != nil {
		return
	}

	lastSeen, err := cfg.Get("", LastSeenVersionKey)
	if err != nil {
		return
	}
	lastSeen = strings.TrimSpace(lastSeen)

	seen, err := version.NewVersion(lastSeen)
	if err != nil || !current.GreaterThan(seen) {
		return
	}

	c := io.Color()
	fmt.Fprintln(io.StdErr, c.Yellow(fmt.Sprintf("What's new in glab %s", currentVersion)))
	fmt.Fprintln(io.StdErr, "  Run: glab whatsnew")

	// An upgrade is the only moment a new bundled-skill payload becomes
	// available, so the skill check piggybacks on the banner.
	writeSkillUpdateLine(io, bundledSkillUpdates(cfg), true)

	if err := SetLastSeenVersion(cfg, currentVersion); err != nil {
		fmt.Fprintf(io.StdErr, "  (could not update %s: %s)\n", LastSeenVersionKey, err)
	}
}

func SetLastSeenVersion(cfg config.Config, v string) error {
	if err := cfg.Set("", LastSeenVersionKey, v); err != nil {
		return err
	}
	return cfg.Write()
}

func isWhatsNewEnabled(cfg config.Config) bool {
	// cfg.Get already consults GLAB_SHOW_WHATS_NEW via EnvKeyEquivalence.
	val, err := cfg.Get("", "show_whats_new")
	if err != nil || val == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "false", "no", "n", "0":
		return false
	}
	return true
}
