// Package skill defines the shared types used by every source of glab
// agent skills (bundled, remote, etc.). It intentionally has no
// dependencies on the source packages — those import this one.
package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// Source identifies where a Skill was loaded from.
type Source string

const (
	SourceBundled Source = "bundled"
	SourceRemote  Source = "remote"
)

// FileName is the canonical filename for a skill, per the Agent Skills
// specification (https://agentskills.io).
const FileName = "SKILL.md"

// Skill is a single agent skill. Files maps each path inside the skill
// directory (relative to the skill root, e.g. "SKILL.md" or
// "scripts/extract.py") to its contents. Discovery-only operations
// (such as `glab skills list`) may return Skill values with Files
// unpopulated; full skill content is only guaranteed after a Get-style
// lookup.
type Skill struct {
	Name        string
	Description string
	Source      Source
	Files       map[string][]byte
}

// SkillFile returns the contents of the canonical SKILL.md for this
// skill, or nil if Files is unpopulated. It is shorthand for
// s.Files[FileName].
func (s Skill) SkillFile() []byte {
	return s.Files[FileName]
}

// ContentHash returns a stable sha256 over a file tree. Empty input
// returns "" so "nothing on disk yet" callsites don't need a special case.
func ContentHash(files map[string][]byte) string {
	if len(files) == 0 {
		return ""
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		// Length-prefix so two different (path, body) splits can't collide.
		fmt.Fprintf(h, "%d:%s\n%d:", len(p), p, len(files[p]))
		h.Write(files[p])
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
