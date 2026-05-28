// Package installed discovers agent skills that the user has previously
// installed via `glab skills install`. It walks the two well-known
// locations (the current project's .agents/skills/ and ~/.agents/skills/)
// and matches each subdirectory against the bundled + remote registries
// so callers can answer "is this skill known to glab?" without having
// to parse anything.
//
// We intentionally don't maintain a manifest file. Detection is purely
// content-based: each installed skill is hashed and compared against
// the source-of-truth at check time. The trade-off is that we can't
// distinguish a user-edited skill from a stale one — both look
// "diverged." The notification surface is honest about that.
package installed

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gitlab.com/gitlab-org/cli/internal/commands/skills/bundled"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/remote"
	"gitlab.com/gitlab-org/cli/internal/commands/skills/skill"
	"gitlab.com/gitlab-org/cli/internal/git"
)

// skillsRelDir mirrors install.skillsRelDir — kept duplicated rather
// than imported to avoid a back-reference into the install command from
// what should be a leaf data package.
var skillsRelDir = filepath.Join(".agents", "skills")

// Scope identifies which well-known location an installed skill came
// from. Reported back so notifications can disambiguate when the same
// skill name exists in both scopes.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeGlobal  Scope = "global"
)

// Skill is one installed-on-disk skill that matches a known glab
// registry entry. Files holds the on-disk contents (one entry per
// regular file under the skill directory, keyed by path relative to
// the skill root). Hash is skill.ContentHash(Files), cached for cheap
// comparison.
type Skill struct {
	Name   string
	Dir    string
	Scope  Scope
	Source skill.Source
	Files  map[string][]byte
	Hash   string
}

// Discover walks both well-known skill locations and returns every
// subdirectory whose name matches a known bundled or remote skill.
// Anything else (user-authored skills with unfamiliar names, dotfiles,
// regular files) is ignored.
//
// Missing locations (no git repo, no ~/.agents/skills/) are not
// errors — they just contribute zero entries.
func Discover() ([]Skill, error) {
	known, err := buildKnownNames()
	if err != nil {
		return nil, err
	}

	var out []Skill
	for _, loc := range candidateLocations() {
		entries, err := readSkillDirs(loc.dir)
		if err != nil {
			return nil, err
		}
		for _, dirName := range entries {
			src, ok := known[dirName]
			if !ok {
				continue
			}
			s, err := readInstalled(loc.dir, dirName, loc.scope, src)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Scope < out[j].Scope
	})
	return out, nil
}

type location struct {
	dir   string
	scope Scope
}

// candidateLocationsFn is overridable for tests so they can point at
// scratch directories instead of the real ~/ and repo root.
var candidateLocationsFn = defaultCandidateLocations

func candidateLocations() []location { return candidateLocationsFn() }

// StubCandidateLocations replaces the discovery walker's locations
// with a single project-scope entry pointing at dir. Restored on
// test cleanup. Exposed for cross-package tests (e.g., the update
// command) that need installed.Discover() to read from a scratch tree.
func StubCandidateLocations(t interface{ Cleanup(func()) }, dir string) {
	old := candidateLocationsFn
	candidateLocationsFn = func() []location {
		return []location{{dir: dir, scope: ScopeProject}}
	}
	t.Cleanup(func() { candidateLocationsFn = old })
}

func defaultCandidateLocations() []location {
	var out []location
	if root, err := git.ToplevelDir(); err == nil {
		out = append(out, location{filepath.Join(root, skillsRelDir), ScopeProject})
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, location{filepath.Join(home, skillsRelDir), ScopeGlobal})
	}
	return out
}

func readSkillDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills directory %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Reserved namespace for any future glab-managed bookkeeping —
		// don't classify it as a skill even if it appears.
		if e.Name() == ".glab" {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

func readInstalled(parent, dirName string, scope Scope, source skill.Source) (Skill, error) {
	skillDir := filepath.Join(parent, dirName)
	files := map[string][]byte{}
	err := filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Canonicalize to forward slashes so the on-disk hash matches
		// what bundled.Get / remote.Get produce (both use embed/path).
		files[filepath.ToSlash(rel)] = content
		return nil
	})
	if err != nil {
		return Skill{}, fmt.Errorf("reading installed skill %s: %w", skillDir, err)
	}
	return Skill{
		Name:   dirName,
		Dir:    skillDir,
		Scope:  scope,
		Source: source,
		Files:  files,
		Hash:   skill.ContentHash(files),
	}, nil
}

// buildKnownNames returns name → source for every skill glab knows
// how to install. Used to skip user-authored directories that happen
// to live in .agents/skills/ but don't correspond to anything we ship.
func buildKnownNames() (map[string]skill.Source, error) {
	out := map[string]skill.Source{}
	bs, err := bundled.All()
	if err != nil {
		return nil, fmt.Errorf("loading bundled skills: %w", err)
	}
	for _, s := range bs {
		out[s.Name] = skill.SourceBundled
	}
	rs, err := remote.All()
	if err != nil {
		return nil, fmt.Errorf("loading remote skills: %w", err)
	}
	for _, s := range rs {
		// Bundled wins if a name somehow appears in both — bundled is
		// the cheaper source to verify against.
		if _, exists := out[s.Name]; !exists {
			out[s.Name] = skill.SourceRemote
		}
	}
	return out, nil
}
