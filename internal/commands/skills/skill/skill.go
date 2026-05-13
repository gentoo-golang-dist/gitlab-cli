// Package skill defines the shared types used by every source of glab
// agent skills (bundled, remote, etc.). It intentionally has no
// dependencies on the source packages — those import this one.
package skill

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
