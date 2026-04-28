//go:build !integration

package skills

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundledSkillsEmbed(t *testing.T) {
	// Verify the embedded filesystem contains the expected files
	content, err := BundledSkills.ReadFile("bundled/glab/SKILL.md")
	require.NoError(t, err)
	assert.NotEmpty(t, content)

	// Verify frontmatter is present
	text := string(content)
	assert.Contains(t, text, "---")
	assert.Contains(t, text, "name: glab")
	assert.Contains(t, text, "description:")
}
