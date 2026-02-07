//go:build !integration

package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/agentskills"
)

// setupTestSkillsDirectory creates a test directory with sample skills
func setupTestSkillsDirectory(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create first skill
	skill1Dir := filepath.Join(tmpDir, "skill-one")
	err := os.Mkdir(skill1Dir, 0o750)
	require.NoError(t, err)

	skill1Content := `---
name: skill-one
description: First test skill
license: MIT
---

# Skill One

This is the body of skill one.`
	err = os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0o600)
	require.NoError(t, err)

	// Create references directory for skill one
	referencesDir := filepath.Join(skill1Dir, "references")
	err = os.Mkdir(referencesDir, 0o750)
	require.NoError(t, err)

	// Add a reference file
	refContent := "This is a reference document."
	err = os.WriteFile(filepath.Join(referencesDir, "reference.txt"), []byte(refContent), 0o600)
	require.NoError(t, err)

	// Create second skill
	skill2Dir := filepath.Join(tmpDir, "skill-two")
	err = os.Mkdir(skill2Dir, 0o750)
	require.NoError(t, err)

	skill2Content := `---
name: skill-two
description: Second test skill
---

# Skill Two

This is the body of skill two.`
	err = os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2Content), 0o600)
	require.NoError(t, err)

	return tmpDir
}

func TestRegisterSkillTools(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)

	// Create a mock root command
	rootCmd := &cobra.Command{
		Use: "glab",
	}

	// Create MCP server with skills
	server := newMCPServer(rootCmd, skillsDir)

	// Verify that the server has the skillDir set
	assert.Equal(t, skillsDir, server.skillDir)

	// The registerSkillTools should have been called during newMCPServer
	// We can't directly test the registered tools without accessing internal server state,
	// but we can verify the skills were loaded
	skills, err := agentskills.ListSkillsInDirectory(skillsDir)
	require.NoError(t, err)
	assert.Len(t, skills, 2)
}

func TestSkillMetadataHandler(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)

	// Load skills
	skills, err := agentskills.ListSkillsInDirectory(skillsDir)
	require.NoError(t, err)

	// Create server and handler
	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd, skillDir: skillsDir}
	handler := server.createSkillMetadataHandler(skills)

	// Create a mock request
	request := mcp.CallToolRequest{}

	// Call the handler
	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the response contains skill metadata
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	// Parse the JSON response
	var skillSummaries []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &skillSummaries)
	require.NoError(t, err)

	// Verify we have both skills
	assert.Len(t, skillSummaries, 2)
	names := []string{skillSummaries[0].Name, skillSummaries[1].Name}
	assert.Contains(t, names, "skill-one")
	assert.Contains(t, names, "skill-two")
}

func TestSkillMarkdownHandler(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)
	skillPath := filepath.Join(skillsDir, "skill-one", "SKILL.md")

	// Create server and handler
	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd, skillDir: skillsDir}
	handler := server.createSkillMarkdownHandler(skillPath)

	// Create a mock request
	request := mcp.CallToolRequest{}

	// Call the handler
	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the response contains the skill content
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	// Verify content includes frontmatter and body (raw format)
	assert.Contains(t, textContent.Text, "---")
	assert.Contains(t, textContent.Text, "name: skill-one")
	assert.Contains(t, textContent.Text, "description: First test skill")
	assert.Contains(t, textContent.Text, "# Skill One")
	assert.Contains(t, textContent.Text, "This is the body of skill one.")
}

func TestSkillMarkdownHandler_Error(t *testing.T) {
	// Create server with non-existent skill path
	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd}
	handler := server.createSkillMarkdownHandler("/nonexistent/SKILL.md")

	// Create a mock request
	request := mcp.CallToolRequest{}

	// Call the handler
	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	// Verify error message
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Error reading skill file")
}

func TestSkillReferenceHandler(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)
	skillDir := filepath.Join(skillsDir, "skill-one")

	// Create server and handler
	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd, skillDir: skillsDir}
	handler := server.createSkillReferenceHandler(skillDir)

	t.Run("read valid reference file", func(t *testing.T) {
		// Create a mock request with filename parameter
		request := mcp.CallToolRequest{
			Params: mcp.CallToolRequestParams{
				Arguments: map[string]interface{}{
					"filename": "reference.txt",
				},
			},
		}

		// Call the handler
		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)

		// Verify the response contains the file content
		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "This is a reference document.", textContent.Text)
	})

	t.Run("missing filename parameter", func(t *testing.T) {
		request := mcp.CallToolRequest{
			Params: mcp.CallToolRequestParams{
				Arguments: map[string]interface{}{},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "filename parameter is required")
	})

	t.Run("file not found", func(t *testing.T) {
		request := mcp.CallToolRequest{
			Params: mcp.CallToolRequestParams{
				Arguments: map[string]interface{}{
					"filename": "nonexistent.txt",
				},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "file not found")
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		// Try to access a file outside the references directory
		request := mcp.CallToolRequest{
			Params: mcp.CallToolRequestParams{
				Arguments: map[string]interface{}{
					"filename": "../SKILL.md",
				},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "invalid file path")
	})

	t.Run("absolute path prevention", func(t *testing.T) {
		// Try to access an absolute path
		request := mcp.CallToolRequest{
			Params: mcp.CallToolRequestParams{
				Arguments: map[string]interface{}{
					"filename": "/etc/passwd",
				},
			},
		}

		result, err := handler(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)

		require.Len(t, result.Content, 1)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "invalid file path")
	})
}

func TestListSkills(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)

	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd, skillDir: skillsDir}

	skills, err := server.listSkills()
	require.NoError(t, err)
	assert.Len(t, skills, 2)

	names := []string{skills[0].Name, skills[1].Name}
	assert.Contains(t, names, "skill-one")
	assert.Contains(t, names, "skill-two")
}

func TestFindSkillPath(t *testing.T) {
	skillsDir := setupTestSkillsDirectory(t)

	rootCmd := &cobra.Command{Use: "glab"}
	server := &mcpServer{rootCmd: rootCmd, skillDir: skillsDir}

	t.Run("find existing skill", func(t *testing.T) {
		path := server.findSkillPath("skill-one")
		assert.NotEmpty(t, path)
		assert.Contains(t, path, "skill-one")
		assert.Contains(t, path, "SKILL.md")
	})

	t.Run("skill not found", func(t *testing.T) {
		path := server.findSkillPath("nonexistent-skill")
		assert.Empty(t, path)
	})
}
