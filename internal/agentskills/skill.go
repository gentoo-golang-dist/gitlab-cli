package agentskills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata represents the YAML frontmatter of a SKILL.md file
type SkillMetadata struct {
	Name          string                 `yaml:"name"`
	Description   string                 `yaml:"description"`
	License       string                 `yaml:"license,omitempty"`
	Compatibility string                 `yaml:"compatibility,omitempty"`
	Metadata      map[string]interface{} `yaml:"metadata,omitempty"`
	AllowedTools  []string               `yaml:"allowed-tools,omitempty"`
}

// SkillContent represents the full content of a SKILL.md file
type SkillContent struct {
	Metadata    SkillMetadata
	Body        string
	Frontmatter map[string]interface{}
}

// ParseSkillFile reads a SKILL.md file and parses its frontmatter and body
func ParseSkillFile(path string) (*SkillContent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skill file: %w", err)
	}

	content := string(data)

	// Split frontmatter from body using "---" delimiters
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid skill file format: missing YAML frontmatter delimiters")
	}

	// parts[0] is empty (before first ---), parts[1] is frontmatter, parts[2] is body
	frontmatterStr := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	// Parse frontmatter as generic map first
	var frontmatter map[string]interface{}
	if err := yaml.Unmarshal([]byte(frontmatterStr), &frontmatter); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Parse frontmatter into SkillMetadata struct
	var metadata SkillMetadata
	if err := yaml.Unmarshal([]byte(frontmatterStr), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse skill metadata: %w", err)
	}

	// Validate metadata
	if err := ValidateSkillMetadata(&metadata); err != nil {
		return nil, err
	}

	return &SkillContent{
		Metadata:    metadata,
		Body:        body,
		Frontmatter: frontmatter,
	}, nil
}

// ValidateSkillMetadata validates the skill metadata according to Agent Skills specification
func ValidateSkillMetadata(metadata *SkillMetadata) error {
	// Validate name (1-64 chars, lowercase alphanumeric + hyphens, no start/end hyphens, no consecutive hyphens)
	if metadata.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if len(metadata.Name) > 64 {
		return fmt.Errorf("skill name must be 64 characters or less, got %d", len(metadata.Name))
	}

	// Name pattern: lowercase alphanumeric + hyphens, no start/end hyphens, no consecutive hyphens
	namePattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	if !namePattern.MatchString(metadata.Name) {
		return fmt.Errorf("skill name must be lowercase alphanumeric with hyphens (no start/end hyphens, no consecutive hyphens): %s", metadata.Name)
	}

	// Validate description (1-1024 chars)
	if metadata.Description == "" {
		return fmt.Errorf("skill description is required")
	}
	if len(metadata.Description) > 1024 {
		return fmt.Errorf("skill description must be 1024 characters or less, got %d", len(metadata.Description))
	}

	return nil
}

// ListSkillsInDirectory scans a directory for SKILL.md files and returns their metadata
func ListSkillsInDirectory(dir string) ([]SkillMetadata, error) {
	if dir == "" {
		return nil, fmt.Errorf("skill directory path is required")
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to access skill directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skill path is not a directory: %s", dir)
	}

	var skills []SkillMetadata

	// Walk through the directory
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for SKILL.md files
		if !info.IsDir() && strings.ToUpper(filepath.Base(path)) == "SKILL.MD" {
			skillContent, err := ParseSkillFile(path)
			if err != nil {
				// Log error but continue processing other skills
				fmt.Fprintf(os.Stderr, "Warning: failed to parse skill file %s: %v\n", path, err)
				return nil
			}
			skills = append(skills, skillContent.Metadata)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan skill directory: %w", err)
	}

	return skills, nil
}
