// Package registry is the aggregate skill registry — it stitches together
// every source of skills (bundled, remote) and exposes a single surface
// for `glab skills list` and `glab skills install` to consume.
//
// The aggregator owns the user-facing "skill not found" message because
// it is the only place that knows about every source. Source-scoped
// not-found errors propagate via `errors.Is` so genuine load/network
// failures are not masked.
package registry

import (
	"errors"
	"fmt"
	"sort"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/remote"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
)

// ErrNotFound is returned by Get when no source has a skill with the
// requested name.
var ErrNotFound = errors.New("skill not found")

// All returns every known skill across every source, sorted by name.
// Bundled skills include their full file contents; remote skills carry
// metadata only — call Get to materialize files.
func All() ([]skill.Skill, error) {
	bundledSkills, err := bundled.All()
	if err != nil {
		return nil, fmt.Errorf("loading bundled skills: %w", err)
	}
	remoteSkills, err := remote.All()
	if err != nil {
		return nil, fmt.Errorf("loading remote skills registry: %w", err)
	}

	all := make([]skill.Skill, 0, len(bundledSkills)+len(remoteSkills))
	all = append(all, bundledSkills...)
	all = append(all, remoteSkills...)
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}

// Get returns the named skill from whichever source provides it, with
// files populated. Returns ErrNotFound (wrapped with a user-facing
// message) if no source has it. Real load/network failures propagate
// unchanged.
func Get(name string) (skill.Skill, error) {
	if s, err := bundled.Get(name); err == nil {
		return s, nil
	} else if !errors.Is(err, bundled.ErrNotFound) {
		return skill.Skill{}, err
	}

	if s, err := remote.Get(name); err == nil {
		return s, nil
	} else if !errors.Is(err, remote.ErrNotFound) {
		return skill.Skill{}, err
	}

	return skill.Skill{}, fmt.Errorf("%w: unknown skill %q. Run 'glab skills list' to see available skills", ErrNotFound, name)
}
