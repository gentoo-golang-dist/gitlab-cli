package update

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/go-version"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// LastSeenVersionKey is the config key that stores the last glab version a
// user has been shown a post-upgrade banner for. Exposed so sibling commands
// (notably whatsnew) can read and update the same marker.
const LastSeenVersionKey = "last_seen_version"

// MaybeShowPostUpgradeBanner prints a one-time nudge pointing at
// `glab whatsnew` whenever the running version is newer than the
// last_seen_version recorded in config. The marker is then advanced so the
// banner won't repeat on subsequent commands until the user upgrades again.
//
// The default last_seen_version is seeded in defaultFor() (see config_mapping.go)
// so the banner can surface to existing users the first time they run the
// release that ships this feature.
func MaybeShowPostUpgradeBanner(f cmdutils.Factory) {
	if !isWhatsNewEnabled(f) {
		return
	}

	buildInfo := f.BuildInfo()
	// Skip for coding agents — the banner is a human-discovery nudge.
	if buildInfo.CodingAgent != "" {
		return
	}

	currentVersion := strings.TrimSpace(buildInfo.Version)
	current, err := version.NewVersion(currentVersion)
	if err != nil {
		// Unparseable version (e.g. "DEV") — don't track or banner.
		return
	}

	cfg := f.Config()
	lastSeen, err := cfg.Get("", LastSeenVersionKey)
	if err != nil {
		return
	}
	lastSeen = strings.TrimSpace(lastSeen)

	seen, err := version.NewVersion(lastSeen)
	if err != nil || !current.GreaterThan(seen) {
		return
	}

	writePostUpgradeBanner(f.IO(), currentVersion)
	_ = SetLastSeenVersion(cfg, currentVersion)
}

// SetLastSeenVersion stores the given version under LastSeenVersionKey so the
// post-upgrade banner won't fire again until the user upgrades past it.
func SetLastSeenVersion(cfg config.Config, v string) error {
	if err := cfg.Set("", LastSeenVersionKey, v); err != nil {
		return err
	}
	return cfg.Write()
}

func writePostUpgradeBanner(io *iostreams.IOStreams, currentVersion string) {
	c := io.Color()
	fmt.Fprintln(io.StdErr, c.Yellow(fmt.Sprintf("What's new in glab %s", currentVersion)))
	fmt.Fprintln(io.StdErr, "  Run: glab whatsnew")
}

func isWhatsNewEnabled(f cmdutils.Factory) bool {
	if envVal, ok := os.LookupEnv("GLAB_SHOW_WHATS_NEW"); ok {
		switch strings.ToUpper(envVal) {
		case "TRUE", "YES", "Y", "1":
			return true
		case "FALSE", "NO", "N", "0":
			return false
		}
	}

	val, err := f.Config().Get("", "show_whats_new")
	if err != nil || val == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "false", "no", "n", "0":
		return false
	}
	return true
}
