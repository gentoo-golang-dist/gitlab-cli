// Package bundled owns the agent skills shipped with glab. It exposes a small
// registry so the install and list subcommands can share a single source of
// truth, and parses YAML frontmatter from each SKILL.md to derive metadata.
//
// The on-disk layout under assets/ follows the Agent Skills specification
// (https://agentskills.io/specification): each top-level directory is a skill
// whose name matches the directory, and must contain a SKILL.md plus any
// optional supporting files (scripts/, references/, assets/, etc.).
package bundled

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

const (
	// FileName is the canonical filename for a skill, per the Agent Skills
	// specification (https://agentskills.io).
	FileName = "SKILL.md"

	assetsDir = "assets"
)

//go:embed all:assets
var fsys embed.FS

// Skill is a single bundled agent skill resolved from the embedded FS.
// Files maps each path inside the skill directory (relative to the skill
// root, e.g. "SKILL.md" or "scripts/extract.py") to its contents.
type Skill struct {
	Name        string
	Description string
	Files       map[string][]byte
}

// SkillFile returns the contents of the canonical SKILL.md for this skill.
// It is shorthand for s.Files[FileName].
func (s Skill) SkillFile() []byte {
	return s.Files[FileName]
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

var (
	loadOnce sync.Once
	loaded   []Skill
	loadErr  error
)

// All returns every bundled skill, sorted by name.
func All() ([]Skill, error) {
	loadOnce.Do(load)
	if loadErr != nil {
		return nil, loadErr
	}
	out := make([]Skill, len(loaded))
	copy(out, loaded)
	return out, nil
}

// Get returns the bundled skill with the given name.
func Get(name string) (Skill, error) {
	skills, err := All()
	if err != nil {
		return Skill{}, err
	}
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("unknown skill %q. Run 'glab skills list' to see available skills", name)
}

func load() {
	entries, err := fs.ReadDir(fsys, assetsDir)
	if err != nil {
		loadErr = fmt.Errorf("reading bundled skills: %w", err)
		return
	}

	skills := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		s, err := loadSkill(entry.Name())
		if err != nil {
			loadErr = err
			return
		}
		skills = append(skills, s)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	loaded = skills
}

func loadSkill(dirName string) (Skill, error) {
	skillRoot := path.Join(assetsDir, dirName)
	files, err := collectFiles(skillRoot)
	if err != nil {
		return Skill{}, fmt.Errorf("reading skill %q: %w", dirName, err)
	}

	skillMD, ok := files[FileName]
	if !ok {
		return Skill{}, fmt.Errorf("skill %q is missing required %s", dirName, FileName)
	}

	fm, err := parseFrontmatter(skillMD)
	if err != nil {
		return Skill{}, fmt.Errorf("parsing frontmatter in %s/%s: %w", dirName, FileName, err)
	}
	// Minimum checks needed for load to function — name keys the registry
	// and must match the directory; description is rendered by `skills list`.
	// Full Agent Skills spec compliance (regex, length caps, etc.) is left to
	// hand-review and upstream tooling.
	if fm.Name == "" {
		return Skill{}, fmt.Errorf("%s/%s: frontmatter is missing 'name'", dirName, FileName)
	}
	if fm.Name != dirName {
		return Skill{}, fmt.Errorf("%s/%s: frontmatter name %q does not match directory %q", dirName, FileName, fm.Name, dirName)
	}
	if fm.Description == "" {
		return Skill{}, fmt.Errorf("%s/%s: frontmatter is missing 'description'", dirName, FileName)
	}

	return Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Files:       files,
	}, nil
}

// collectFiles walks the embedded skill directory and returns every regular
// file keyed by its path relative to the skill root.
//
// Paths are canonicalized with path.Clean and verified to be rooted under
// skillRoot before the file is read. embed.FS doesn't expose symlinks or
// allow ".." references at build time, so today this is defensive coding
// rather than a real attack surface — but it documents the invariant and
// guards against future refactors that swap embed.FS for a real
// filesystem source.
func collectFiles(skillRoot string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(fsys, skillRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		cleaned := path.Clean(p)
		rel, err := relPath(skillRoot, cleaned)
		if err != nil {
			return err
		}
		content, err := fs.ReadFile(fsys, cleaned)
		if err != nil {
			return err
		}
		files[rel] = content
		return nil
	})
	return files, err
}

// relPath returns full's path relative to root, rejecting anything that
// would resolve outside root (e.g. "..", absolute paths, or sibling
// directories). full must already be cleaned via path.Clean.
func relPath(root, full string) (string, error) {
	if full == root {
		return "", fmt.Errorf("unexpected: file path %q equals skill root", full)
	}
	if !strings.HasPrefix(full, root+"/") {
		return "", fmt.Errorf("path %q is not under skill root %q", full, root)
	}
	return full[len(root)+1:], nil
}

// parseFrontmatter extracts the YAML block between the first pair of `---`
// delimiters at the top of a SKILL.md.
func parseFrontmatter(content []byte) (frontmatter, error) {
	var fm frontmatter
	const delim = "---"

	trimmed := bytes.TrimLeft(content, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(delim)) {
		return fm, fmt.Errorf("missing leading '---' delimiter")
	}
	rest := trimmed[len(delim):]
	nl := bytes.IndexByte(rest, '\n')
	if nl == -1 {
		return fm, fmt.Errorf("missing newline after opening '---'")
	}
	rest = rest[nl+1:]

	before, _, ok := bytes.Cut(rest, []byte("\n"+delim))
	if !ok {
		return fm, fmt.Errorf("missing closing '---' delimiter")
	}

	if err := yaml.Unmarshal(before, &fm); err != nil {
		return fm, err
	}
	return fm, nil
}
