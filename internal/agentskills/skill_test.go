//go:build !integration

package agentskills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSkillFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		errorMsg    string
		validate    func(*testing.T, *SkillContent)
	}{
		{
			name: "valid SKILL.md with all fields",
			content: `---
name: test-skill
description: A test skill for validation
license: MIT
compatibility: ">=1.0.0"
metadata:
  author: Test Author
  version: 1.0.0
allowed-tools:
  - tool1
  - tool2
---

# Test Skill

This is the body of the skill.`,
			expectError: false,
			validate: func(t *testing.T, sc *SkillContent) {
				assert.Equal(t, "test-skill", sc.Metadata.Name)
				assert.Equal(t, "A test skill for validation", sc.Metadata.Description)
				assert.Equal(t, "MIT", sc.Metadata.License)
				assert.Equal(t, ">=1.0.0", sc.Metadata.Compatibility)
				assert.Equal(t, "# Test Skill\n\nThis is the body of the skill.", sc.Body)
				assert.Len(t, sc.Metadata.AllowedTools, 2)
				assert.Contains(t, sc.Metadata.AllowedTools, "tool1")
			},
		},
		{
			name: "minimal SKILL.md with only required fields",
			content: `---
name: minimal-skill
description: Minimal skill with only required fields
---

Body content here.`,
			expectError: false,
			validate: func(t *testing.T, sc *SkillContent) {
				assert.Equal(t, "minimal-skill", sc.Metadata.Name)
				assert.Equal(t, "Minimal skill with only required fields", sc.Metadata.Description)
				assert.Equal(t, "", sc.Metadata.License)
				assert.Equal(t, "Body content here.", sc.Body)
			},
		},
		{
			name: "invalid frontmatter - missing name",
			content: `---
description: Missing name field
---

Body.`,
			expectError: true,
			errorMsg:    "skill name is required",
		},
		{
			name: "invalid frontmatter - missing description",
			content: `---
name: no-description
---

Body.`,
			expectError: true,
			errorMsg:    "skill description is required",
		},
		{
			name: "malformed YAML",
			content: `---
name: test
description: [invalid yaml structure
---

Body.`,
			expectError: true,
			errorMsg:    "failed to parse YAML frontmatter",
		},
		{
			name: "missing frontmatter delimiters",
			content: `name: test
description: No delimiters

Body.`,
			expectError: true,
			errorMsg:    "missing YAML frontmatter delimiters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			skillFile := filepath.Join(tmpDir, "SKILL.md")
			err := os.WriteFile(skillFile, []byte(tt.content), 0o600)
			require.NoError(t, err)

			// Parse the file
			result, err := ParseSkillFile(skillFile)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestValidateSkillMetadata(t *testing.T) {
	tests := []struct {
		name        string
		metadata    SkillMetadata
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid name and description",
			metadata: SkillMetadata{
				Name:        "valid-skill-name",
				Description: "A valid description",
			},
			expectError: false,
		},
		{
			name: "valid name with numbers",
			metadata: SkillMetadata{
				Name:        "skill-123",
				Description: "Description",
			},
			expectError: false,
		},
		{
			name: "invalid name - uppercase",
			metadata: SkillMetadata{
				Name:        "Invalid-Name",
				Description: "Description",
			},
			expectError: true,
			errorMsg:    "must be lowercase alphanumeric",
		},
		{
			name: "invalid name - starting with hyphen",
			metadata: SkillMetadata{
				Name:        "-invalid",
				Description: "Description",
			},
			expectError: true,
			errorMsg:    "must be lowercase alphanumeric",
		},
		{
			name: "invalid name - ending with hyphen",
			metadata: SkillMetadata{
				Name:        "invalid-",
				Description: "Description",
			},
			expectError: true,
			errorMsg:    "must be lowercase alphanumeric",
		},
		{
			name: "invalid name - consecutive hyphens",
			metadata: SkillMetadata{
				Name:        "invalid--name",
				Description: "Description",
			},
			expectError: true,
			errorMsg:    "must be lowercase alphanumeric",
		},
		{
			name: "invalid name - too long",
			metadata: SkillMetadata{
				Name:        "this-is-a-very-long-skill-name-that-exceeds-the-maximum-allowed-length-of-sixty-four-characters",
				Description: "Description",
			},
			expectError: true,
			errorMsg:    "must be 64 characters or less",
		},
		{
			name: "invalid description - empty",
			metadata: SkillMetadata{
				Name:        "valid-name",
				Description: "",
			},
			expectError: true,
			errorMsg:    "description is required",
		},
		{
			name: "invalid description - too long",
			metadata: SkillMetadata{
				Name:        "valid-name",
				Description: string(make([]byte, 1025)),
			},
			expectError: true,
			errorMsg:    "must be 1024 characters or less",
		},
		{
			name: "valid description at max length",
			metadata: SkillMetadata{
				Name:        "valid-name",
				Description: string(make([]byte, 1024)),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillMetadata(&tt.metadata)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListSkillsInDirectory(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) error
		expectError bool
		errorMsg    string
		validate    func(*testing.T, []SkillMetadata)
	}{
		{
			name: "single skill",
			setup: func(dir string) error {
				skillContent := `---
name: skill-one
description: First skill
---

Body.`
				return os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillContent), 0o600)
			},
			expectError: false,
			validate: func(t *testing.T, skills []SkillMetadata) {
				assert.Len(t, skills, 1)
				assert.Equal(t, "skill-one", skills[0].Name)
			},
		},
		{
			name: "multiple skills",
			setup: func(dir string) error {
				skill1 := `---
name: skill-one
description: First skill
---

Body 1.`
				skill2 := `---
name: skill-two
description: Second skill
---

Body 2.`
				subDir := filepath.Join(dir, "subdir")
				if err := os.Mkdir(subDir, 0o750); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill1), 0o600); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(subDir, "SKILL.md"), []byte(skill2), 0o600)
			},
			expectError: false,
			validate: func(t *testing.T, skills []SkillMetadata) {
				assert.Len(t, skills, 2)
				names := []string{skills[0].Name, skills[1].Name}
				assert.Contains(t, names, "skill-one")
				assert.Contains(t, names, "skill-two")
			},
		},
		{
			name: "no skills",
			setup: func(dir string) error {
				// Create some non-skill files
				return os.WriteFile(filepath.Join(dir, "README.md"), []byte("Not a skill"), 0o600)
			},
			expectError: false,
			validate: func(t *testing.T, skills []SkillMetadata) {
				assert.Len(t, skills, 0)
			},
		},
		{
			name: "nested directories",
			setup: func(dir string) error {
				skill := `---
name: nested-skill
description: Nested skill
---

Body.`
				nestedDir := filepath.Join(dir, "level1", "level2")
				if err := os.MkdirAll(nestedDir, 0o750); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(nestedDir, "SKILL.md"), []byte(skill), 0o600)
			},
			expectError: false,
			validate: func(t *testing.T, skills []SkillMetadata) {
				assert.Len(t, skills, 1)
				assert.Equal(t, "nested-skill", skills[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.setup != nil {
				err := tt.setup(tmpDir)
				require.NoError(t, err)
			}

			result, err := ListSkillsInDirectory(tmpDir)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestListSkillsInDirectory_Errors(t *testing.T) {
	t.Run("empty directory path", func(t *testing.T) {
		_, err := ListSkillsInDirectory("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skill directory path is required")
	})

	t.Run("non-existent directory", func(t *testing.T) {
		_, err := ListSkillsInDirectory("/nonexistent/path")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to access skill directory")
	})

	t.Run("path is not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		err := os.WriteFile(filePath, []byte("content"), 0o600)
		require.NoError(t, err)

		_, err = ListSkillsInDirectory(filePath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}
