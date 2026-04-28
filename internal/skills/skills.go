package skills

import "embed"

// BundledSkills contains all bundled SKILL.md files.
// The directory structure under bundled/ maps directly to the
// target installation layout: bundled/glab/SKILL.md -> <target>/glab/SKILL.md
//
//go:embed bundled/glab/SKILL.md
var BundledSkills embed.FS
