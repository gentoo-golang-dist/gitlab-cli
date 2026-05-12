// Package bundled owns the agent skills shipped with glab. It exposes a small
// registry so the install and list subcommands can share a single source of
// truth, and parses YAML frontmatter from each SKILL.md to derive metadata.
package bundled

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"sync"

	"go.yaml.in/yaml/v3"
)

const (
	// FileName is the canonical filename for a skill, per the Agent Skills
	// specification (https://agentskills.io).
	FileName = "SKILL.md"

	assetsDir = "assets"
)

//go:embed all:assets/*/SKILL.md
var fsys embed.FS

// Skill is a single bundled agent skill resolved from the embedded FS.
type Skill struct {
	Name        string
	Description string
	Content     []byte
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
		dirName := entry.Name()
		filePath := path.Join(assetsDir, dirName, FileName)
		content, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			loadErr = fmt.Errorf("reading %s: %w", filePath, err)
			return
		}

		fm, err := parseFrontmatter(content)
		if err != nil {
			loadErr = fmt.Errorf("parsing frontmatter in %s: %w", filePath, err)
			return
		}
		if fm.Name == "" {
			loadErr = fmt.Errorf("%s: frontmatter is missing 'name'", filePath)
			return
		}
		if fm.Name != dirName {
			loadErr = fmt.Errorf("%s: frontmatter name %q does not match directory %q", filePath, fm.Name, dirName)
			return
		}

		skills = append(skills, Skill{
			Name:        fm.Name,
			Description: fm.Description,
			Content:     content,
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	loaded = skills
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
	// require a newline after the opening delimiter
	nl := bytes.IndexByte(rest, '\n')
	if nl == -1 {
		return fm, fmt.Errorf("missing newline after opening '---'")
	}
	rest = rest[nl+1:]

	before, _, ok := bytes.Cut(rest, []byte("\n"+delim))
	if !ok {
		return fm, fmt.Errorf("missing closing '---' delimiter")
	}
	block := before

	if err := yaml.Unmarshal(block, &fm); err != nil {
		return fm, err
	}
	return fm, nil
}
