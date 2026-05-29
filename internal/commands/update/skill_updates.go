package update

import (
	"fmt"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/installed"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/remote"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// Overridable so tests don't pick up the developer's ~/.agents/skills/.
var discoverInstalled = installed.Discover

// bundledSkillUpdates lists installed bundled skills whose on-disk content
// does not match the version embedded in this binary.
func bundledSkillUpdates(cfg config.Config) []string {
	return skillUpdates(cfg, skill.SourceBundled, bundled.Get)
}

// remoteSkillUpdates lists installed remote skills whose on-disk content
// does not match the current upstream. Each name triggers a gitlab.com
// request — gate on the 24h CheckUpdate cadence, not per command.
func remoteSkillUpdates(cfg config.Config) []string {
	return skillUpdates(cfg, skill.SourceRemote, remote.Get)
}

// skillUpdates is best-effort: discovery or getter failures return nil
// rather than an error so a stale check can't disrupt the user's command.
func skillUpdates(cfg config.Config, source skill.Source, getSource func(string) (skill.Skill, error)) []string {
	if !isSkillNotificationsEnabled(cfg) {
		return nil
	}
	all, err := discoverInstalled()
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, ins := range all {
		if ins.Source != source || seen[ins.Name] {
			continue
		}
		src, err := getSource(ins.Name)
		if err != nil {
			continue
		}
		if skill.ContentHash(src.Files) != ins.Hash {
			out = append(out, ins.Name)
			seen[ins.Name] = true
		}
	}
	return out
}

func writeSkillUpdateLine(io *iostreams.IOStreams, names []string, indent bool) {
	if len(names) == 0 {
		return
	}
	prefix := ""
	if indent {
		prefix = "  "
	}
	noun := "skill has"
	action := fmt.Sprintf("glab skills update %s", names[0])
	if len(names) > 1 {
		noun = "skills have"
		action = "glab skills update --all"
	}
	fmt.Fprintf(io.StdErr, "%s%d installed agent %s updates: %s. Run: %s\n",
		prefix, len(names), noun, strings.Join(names, ", "), action)
}

func isSkillNotificationsEnabled(cfg config.Config) bool {
	// cfg.Get already consults GLAB_NOTIFY_SKILL_UPDATES via EnvKeyEquivalence.
	val, err := cfg.Get("", "notify_skill_updates")
	if err != nil || val == "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "false", "no", "n", "0":
		return false
	}
	return true
}
